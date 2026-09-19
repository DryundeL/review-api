package authhttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"review-api/internal/modules/auth"
	"review-api/internal/platform/httpserver"
)

func TestIdentityUnauthorized(t *testing.T) {
	t.Parallel()
	mod := auth.New(auth.Dependencies{BotToken: "tok", MaxAge: time.Hour})

	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	mod.RegisterHTTP(e.Group("/v1"))

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/identity", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "UNAUTHORIZED" {
		t.Fatalf("body %+v", body)
	}
}
