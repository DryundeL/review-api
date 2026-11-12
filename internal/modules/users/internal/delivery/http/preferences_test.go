package usershttp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users/internal/application/command"
	"review-api/internal/modules/users/internal/domain"
	"review-api/internal/platform/httpserver"
)

type preferenceStore struct{ types []string }

func (s *preferenceStore) GetContentPreferences(context.Context, uuid.UUID) (domain.ContentPreferences, error) {
	types := append([]string{}, s.types...)
	return domain.ContentPreferences{Types: types, NeedsOnboarding: len(types) == 0}, nil
}
func (s *preferenceStore) SaveContentPreferences(_ context.Context, _ uuid.UUID, types []string) (domain.ContentPreferences, error) {
	s.types = types
	return domain.ContentPreferences{Types: types}, nil
}

func TestContentPreferencesHTTP(t *testing.T) {
	store := &preferenceStore{}
	h := NewHandler(nil, nil, nil, stubUsers{id: uuid.New()}, nil)
	h.SetContentPreferences(command.NewContentPreferencesHandler(store))
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	auth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Authorization") != "" {
				c.SetRequest(c.Request().WithContext(authcontract.WithIdentity(c.Request().Context(), authcontract.TelegramIdentity{TelegramID: 42})))
			}
			return next(c)
		}
	}
	Register(e.Group("/v1"), h, auth)
	request := func(method, path, body string, authorized bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authorized {
			req.Header.Set("Authorization", "Bearer test")
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	if r := request("GET", "/v1/content-preferences", "", false); r.Code != 401 {
		t.Fatalf("unauthorized: %d", r.Code)
	}
	if r := request("GET", "/v1/content-preferences", "", true); r.Code != 200 || !strings.Contains(r.Body.String(), `"needsOnboarding":true`) {
		t.Fatalf("initial: %s", r.Body)
	}
	for _, body := range []string{`{}`, `{"types":[]}`, `{"types":["manga","manga"]}`, `{"types":["../bad"]}`, `{"types":["manga"],"userId":"other"}`, `{"types":["manga"]} {}`} {
		r := request("PUT", "/v1/content-preferences", body, true)
		if r.Code != 400 && r.Code != 422 {
			t.Fatalf("accepted %s: %d", body, r.Code)
		}
	}
	if r := request("PUT", "/v1/content-preferences", `{"types":["manga"]}`, true); r.Code != 200 || !strings.Contains(r.Body.String(), `"needsOnboarding":false`) {
		t.Fatalf("save: %d %s", r.Code, r.Body)
	}
	if r := request("GET", "/v1/content-preferences", "", true); r.Code != 200 || !strings.Contains(r.Body.String(), `"types":["manga"]`) || !strings.Contains(r.Body.String(), `"needsOnboarding":false`) {
		t.Fatalf("saved preferences: %d %s", r.Code, r.Body)
	}
}
