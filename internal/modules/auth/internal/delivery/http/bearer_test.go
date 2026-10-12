package authhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"review-api/internal/modules/auth/contract"
	"review-api/internal/modules/auth/internal/domain"
)

func TestBearerAcceptsJWT(t *testing.T) {
	t.Parallel()
	secret := []byte(strings.Repeat("s", 32))
	token, err := domain.SignJWT(secret, 42, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(nil, nil, secret)
	e := echo.New()
	e.GET("/v1/profile", func(c echo.Context) error {
		id, err := contract.IdentityFromContext(c.Request().Context())
		if err != nil || id.TelegramID != 42 {
			t.Fatalf("identity %+v err=%v", id, err)
		}
		return c.NoContent(http.StatusNoContent)
	}, h.Bearer)

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}
