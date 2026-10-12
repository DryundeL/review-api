package usershttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users/internal/application/query"
	"review-api/internal/modules/users/internal/domain"
	"review-api/internal/platform/httpserver"
)

type stubProfiles struct {
	user domain.User
}

func (s stubProfiles) FindByTelegramID(context.Context, int64) (domain.User, error) {
	return s.user, nil
}

func TestProfile(t *testing.T) {
	t.Parallel()
	h := NewHandler(query.NewGetProfileHandler(stubProfiles{user: domain.User{
		ID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TelegramID: 42,
		Username:   "ann",
		FirstName:  "Ann",
	}}))
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	e.GET("/v1/profile", h.Profile, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 42})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.FirstName != "Ann" || body.Username != "ann" || body.TelegramID != 42 {
		t.Fatalf("body = %+v", body)
	}

	e.GET("/open", h.Profile)
	req = httptest.NewRequest(http.MethodGet, "/open", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("open status %d", rec.Code)
	}
}
