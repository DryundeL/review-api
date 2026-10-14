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

	authcontract "review-api/internal/modules/auth/contract"
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

func (memWriter) Save(context.Context, domain.Work) error { return nil }

type memTypes struct{}

func (memTypes) RequireActive(context.Context, []string) error { return nil }

type memAuthors struct{}

func (memAuthors) UserID(context.Context, int64) (uuid.UUID, error) {
	return uuid.MustParse("33333333-3333-3333-3333-333333333333"), nil
}

type memPublisher struct{}

func (memPublisher) Publish(context.Context, string, string, any) error { return nil }

func TestCreateRequiresAuthAndRejectsBadCover(t *testing.T) {
	t.Parallel()
	h := NewHandler(
		command.NewCreateWorkHandler(memWriter{}, memTx{}, memTypes{}, memAuthors{}, memPublisher{}),
		query.NewGetWorkHandler(missingWorks{}),
		query.NewSearchWorksHandler(missingWorks{}),
	)
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	Register(e.Group("/v1"), h, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Authorization") == "" {
				return next(c)
			}
			ctx := authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 7})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/works", strings.NewReader(`{"title":"Фильм","primaryType":"movie","releaseStatus":"completed"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/works", strings.NewReader(`{"title":"Фильм","primaryType":"movie","releaseStatus":"completed","coverUrl":"https://10.0.0.1/a.jpg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/works", strings.NewReader(`{"title":"Фильм","primaryType":"movie","releaseStatus":"completed"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body workJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Origin != "user" || body.PrimaryType != "movie" || body.CreatedBy != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("body = %+v", body)
	}
}

type missingWorks struct{}

func (missingWorks) Get(context.Context, uuid.UUID) (ports.Work, error) {
	return ports.Work{}, contract.ErrNotFound
}

func (missingWorks) Search(context.Context, string, string, int) ([]ports.Work, error) {
	return nil, nil
}
