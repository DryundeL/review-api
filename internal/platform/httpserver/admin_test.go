package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAdminToken(t *testing.T) {
	t.Parallel()
	token := strings.Repeat("a", 32)
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/v1/admin/review-types", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, AdminToken(token))

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/review-types", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing header status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/review-types", nil)
	req.Header.Set(AdminTokenHeader, token+"x")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/review-types", nil)
	req.Header.Set(AdminTokenHeader, token)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid token status %d", rec.Code)
	}
}

func TestAdminTokenRejectsShortSecret(t *testing.T) {
	t.Parallel()
	e := echo.New()
	e.HTTPErrorHandler = ErrorHandler
	e.GET("/admin", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, AdminToken("short"))

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set(AdminTokenHeader, "short")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}
