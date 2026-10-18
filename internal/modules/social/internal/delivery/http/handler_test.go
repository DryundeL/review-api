package socialhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/social/internal/application/ports"
	"review-api/internal/platform/httpserver"
)

func TestFollowItemsJSON(t *testing.T) {
	t.Parallel()
	uid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	items := followItems([]ports.FollowUser{{
		UserID: uid, Username: "ann", PhotoURL: "https://t.me/i/userpic/320/a.jpg",
		IsFollowing: true, CreatedAt: time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC),
	}})
	if len(items) != 1 || items[0].Username != "ann" || !items[0].IsFollowing {
		t.Fatalf("items = %+v", items)
	}
	if _, err := json.Marshal(listResponse{Items: items}); err != nil {
		t.Fatal(err)
	}
}

func TestListRejectsBadProfileID(t *testing.T) {
	t.Parallel()
	h := &Handler{}
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	e.GET("/v1/profiles/:id/followers", h.ListFollowers)

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles/not-a-uuid/followers", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
}

func TestParsePage(t *testing.T) {
	t.Parallel()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/x?limit=10&offset=30", nil)
	c := e.NewContext(req, httptest.NewRecorder())
	limit, offset, err := parsePage(c)
	if err != nil || limit != 10 || offset != 30 {
		t.Fatalf("got %d %d %v", limit, offset, err)
	}
}
