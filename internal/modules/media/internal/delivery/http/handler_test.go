package mediahttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/command"
	"review-api/internal/modules/media/internal/application/ports"
	"review-api/internal/modules/media/internal/infrastructure/storage"
	"review-api/internal/platform/httpserver"
)

var ownerID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

type fakeUsers struct{}

func (fakeUsers) UserID(context.Context, int64) (uuid.UUID, error) { return ownerID, nil }

type fakeRepo struct {
	items []ports.ImageRecord
	err   error
}

func (r *fakeRepo) Insert(_ context.Context, item ports.ImageRecord) error {
	if r.err != nil {
		return r.err
	}
	r.items = append(r.items, item)
	return nil
}

func (r *fakeRepo) LockUnattached(context.Context, time.Time, int) ([]ports.ImageRecord, error) {
	return nil, nil
}

func (r *fakeRepo) DeleteByIDs(context.Context, []uuid.UUID) error { return nil }

func (r *fakeRepo) Attach(context.Context, uuid.UUID, []uuid.UUID) ([]ports.ImageRecord, error) {
	return nil, nil
}

type fakeQuota struct{ allow bool }

func (q fakeQuota) Allow(context.Context, uuid.UUID) (bool, error) { return q.allow, nil }

type env struct {
	e    *echo.Echo
	repo *fakeRepo
	dir  string
}

func newEnv(t *testing.T, allow bool) env {
	t.Helper()
	dir := t.TempDir()
	repo := &fakeRepo{}
	upload := command.NewUploadImageHandler(repo, storage.NewLocalStore(dir), fakeQuota{allow: allow})
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get(echo.HeaderAuthorization) == "" {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			ctx := authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 42})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
	Register(e.Group("/v1"), NewHandler(upload, fakeUsers{}, "/storage/"), auth)
	return env{e: e, repo: repo, dir: dir}
}

func multipartBody(t *testing.T, field, filename, contentType string, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("note", "ignored")
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	h["Content-Type"] = []string{contentType}
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

func (en env) do(t *testing.T, body *bytes.Buffer, contentType string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/media/images", body)
	req.Header.Set(echo.HeaderContentType, contentType)
	if authed {
		req.Header.Set(echo.HeaderAuthorization, "Bearer test")
	}
	rec := httptest.NewRecorder()
	en.e.ServeHTTP(rec, req)
	return rec
}

func pngWithText(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	src := buf.Bytes()
	iend := len(src) - 12
	// tEXt "secret" with a deliberately wrong CRC: the chunk is dropped before decoding.
	chunk := append([]byte{0, 0, 0, 6}, []byte("tEXtsecret\x00\x00\x00\x00")...)
	return append(append(append([]byte{}, src[:iend]...), chunk...), src[iend:]...)
}

func TestUploadImageStoresCleanFile(t *testing.T) {
	t.Parallel()
	en := newEnv(t, true)
	body, ct := multipartBody(t, "file", "../../evil.svg", "image/svg+xml", pngWithText(t))
	rec := en.do(t, body, ct, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
	var resp imageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(en.repo.items) != 1 {
		t.Fatalf("records = %d", len(en.repo.items))
	}
	item := en.repo.items[0]
	if item.OwnerID != ownerID || item.ContentType != "image/png" || item.Width != 3 || item.Height != 2 {
		t.Fatalf("record = %+v", item)
	}
	if resp.URL != "/storage/"+item.StorageKey || !strings.HasSuffix(resp.URL, ".png") || resp.ID != item.ID.String() {
		t.Fatalf("response = %+v", resp)
	}
	stored, err := os.ReadFile(filepath.Join(en.dir, filepath.FromSlash(item.StorageKey)))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("secret")) || len(stored) != item.SizeBytes {
		t.Fatal("stored file still has metadata or wrong size")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store")
	}
}

func TestUploadImageErrors(t *testing.T) {
	t.Parallel()
	valid := pngWithText(t)
	cases := []struct {
		name   string
		allow  bool
		authed bool
		field  string
		data   []byte
		want   int
	}{
		{"guest", true, false, "file", valid, http.StatusUnauthorized},
		{"gif disguised as png", true, true, "file", []byte("GIF89a\x01\x00\x01\x00"), http.StatusUnsupportedMediaType},
		{"svg", true, true, "file", []byte(`<svg onload="alert(1)"/>`), http.StatusUnsupportedMediaType},
		{"broken png", true, true, "file", valid[:30], http.StatusUnprocessableEntity},
		{"missing file part", true, true, "image", valid, http.StatusUnprocessableEntity},
		{"over 300 KB", true, true, "file", append(append([]byte{}, valid...), make([]byte, contract.MaxImageBytes)...), http.StatusRequestEntityTooLarge},
		{"body over request limit", true, true, "file", make([]byte, maxRequestBytes+1), http.StatusRequestEntityTooLarge},
		{"quota", false, true, "file", valid, http.StatusTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			en := newEnv(t, tc.allow)
			body, ct := multipartBody(t, tc.field, "a.png", "image/png", tc.data)
			rec := en.do(t, body, ct, tc.authed)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tc.want, rec.Body.String())
			}
			if len(en.repo.items) != 0 {
				t.Fatal("nothing must be recorded")
			}
			if entries, _ := os.ReadDir(filepath.Join(en.dir, "images")); len(entries) != 0 {
				t.Fatalf("files left: %v", entries)
			}
		})
	}
}

func TestUploadImageNotMultipart(t *testing.T) {
	t.Parallel()
	en := newEnv(t, true)
	rec := en.do(t, bytes.NewBufferString(`{"file":"x"}`), echo.MIMEApplicationJSON, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUploadImageRemovesFileWhenInsertFails(t *testing.T) {
	t.Parallel()
	en := newEnv(t, true)
	en.repo.err = errors.New("db down")
	body, ct := multipartBody(t, "file", "a.png", "image/png", pngWithText(t))
	rec := en.do(t, body, ct, true)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "db down") {
		t.Fatal("internal error leaked")
	}
	if entries, _ := os.ReadDir(filepath.Join(en.dir, "images")); len(entries) != 0 {
		t.Fatalf("orphan file left: %v", entries)
	}
}
