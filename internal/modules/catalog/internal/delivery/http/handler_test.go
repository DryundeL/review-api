package cataloghttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/command"
	"review-api/internal/modules/catalog/internal/application/ports"
	"review-api/internal/modules/catalog/internal/application/query"
	"review-api/internal/modules/catalog/internal/domain"
	"review-api/internal/platform/httpserver"
)

type memTx struct{}

func (memTx) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type memWriter struct{}

func (memWriter) Save(context.Context, domain.MediaObject) error { return nil }

type memTypes struct{}

func (memTypes) RequireActive(context.Context, []string) error { return nil }

type memPublisher struct{}

func (memPublisher) Publish(context.Context, string, string, any) error { return nil }

func TestImportRequiresAdminAndRejectsBadPoster(t *testing.T) {
	t.Parallel()
	h := NewHandler(
		command.NewImportMediaObjectsHandler(memWriter{}, memTx{}, memTypes{}, memPublisher{}),
		query.NewGetMediaObjectHandler(missingItems{}),
		query.NewSearchMediaObjectsHandler(missingItems{}),
	)
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	token := strings.Repeat("a", 32)
	Register(e.Group("/v1"), h, httpserver.AdminToken(token))

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/media-objects/import", strings.NewReader(`{"items":[{"title":"Фильм","type":"movie","releaseStatus":"completed"}]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/media-objects/import", strings.NewReader(`{"items":[{"title":"Фильм","type":"movie","releaseStatus":"completed","posterUrl":"https://10.0.0.1/a.jpg"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpserver.AdminTokenHeader, token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/media-objects/import", strings.NewReader(`{"items":[{"title":"Фильм","originalTitle":"Movie","type":"movie","genre":"drama","releaseStatus":"completed"}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpserver.AdminTokenHeader, token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body importResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || len(body.Items) != 1 || body.Items[0].Type != "movie" || body.Items[0].OriginalTitle != "Movie" || body.Items[0].Info.Genre != "drama" {
		t.Fatalf("body = %+v", body)
	}
}

func TestImportCSV(t *testing.T) {
	t.Parallel()
	h := NewHandler(
		command.NewImportMediaObjectsHandler(memWriter{}, memTx{}, memTypes{}, memPublisher{}),
		query.NewGetMediaObjectHandler(missingItems{}),
		query.NewSearchMediaObjectsHandler(missingItems{}),
	)
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	token := strings.Repeat("b", 32)
	Register(e.Group("/v1"), h, httpserver.AdminToken(token))

	csvBody := "title,original_title,type,genre,release_status,release_year,poster_url\nМатрица,The Matrix,movie,sci-fi,completed,1999,https://cdn.example/p.jpg\n"
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/media-objects/import", strings.NewReader(csvBody))
	req.Header.Set("Content-Type", "text/csv")
	req.Header.Set(httpserver.AdminTokenHeader, token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body importResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 1 || body.Items[0].OriginalTitle != "The Matrix" || body.Items[0].Info.Genre != "sci-fi" {
		t.Fatalf("body = %+v", body)
	}
}

type missingItems struct{}

func (missingItems) Get(context.Context, uuid.UUID) (ports.MediaObject, error) {
	return ports.MediaObject{}, contract.ErrNotFound
}

func (missingItems) Search(context.Context, string, string, int) ([]ports.MediaObject, error) {
	return nil, nil
}

func TestImportNestedInfo(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		status                  int
	}{
		{"json", "application/json", `{"items":[{"title":"Matrix","type":"movie","info":{"genre":"sci-fi","releaseYear":1999,"releaseStatus":"completed"}}]}`, 201},
		{"csv", "text/csv", "title,type,info\nMatrix,movie,\"{\"\"genre\"\":\"\"sci-fi\"\",\"\"releaseYear\"\":1999,\"\"releaseStatus\"\":\"\"completed\"\"}\"\n", 201},
		{"invalid year", "application/json", `{"items":[{"title":"Matrix","type":"movie","info":{"releaseYear":1700,"releaseStatus":"completed"}}]}`, 422},
		{"invalid status", "application/json", `{"items":[{"title":"Matrix","type":"movie","info":{"releaseStatus":"unknown"}}]}`, 422},
		{"mixed", "application/json", `{"items":[{"title":"Matrix","type":"movie","genre":"drama","info":{"releaseStatus":"completed"}}]}`, 422},
		{"null csv info", "text/csv", "title,type,info\nMatrix,movie,null\n", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(command.NewImportMediaObjectsHandler(memWriter{}, memTx{}, memTypes{}, memPublisher{}), nil, nil)
			e := echo.New()
			e.HTTPErrorHandler = httpserver.ErrorHandler
			e.POST("/import", h.Import)
			req := httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if tc.status == 201 {
				var body struct {
					Items []struct {
						Info domain.Info `json:"info"`
					} `json:"items"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if len(body.Items) != 1 || body.Items[0].Info.Genre != "sci-fi" || body.Items[0].Info.ReleaseYear == nil || *body.Items[0].Info.ReleaseYear != 1999 {
					t.Fatalf("body=%s", rec.Body.String())
				}
				var raw map[string]any
				json.Unmarshal(rec.Body.Bytes(), &raw)
				item := raw["items"].([]any)[0].(map[string]any)
				if _, exists := item["releaseStatus"]; exists {
					t.Fatal("legacy field in response")
				}
			}
		})
	}
}
