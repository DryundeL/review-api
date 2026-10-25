package reviewshttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/application/query"
	"review-api/internal/platform/httpserver"
)

type topReader struct {
	review  ports.Review
	mediaID uuid.UUID
	err     error
	called  bool
}

func (r *topReader) Get(context.Context, uuid.UUID, uuid.UUID) (ports.Review, error) {
	return r.review, r.err
}
func (r *topReader) ListByMedia(context.Context, uuid.UUID, uuid.UUID, int) ([]ports.Review, error) {
	panic("unexpected ListByMedia")
}
func (r *topReader) ListByAuthor(context.Context, uuid.UUID, uuid.UUID, bool, int) ([]ports.Review, error) {
	panic("unexpected ListByAuthor")
}
func (r *topReader) TopByMedia(_ context.Context, mediaID, viewerID uuid.UUID) ([]ports.Review, error) {
	r.called = true
	r.mediaID = mediaID
	return nil, r.err
}

func TestTopReviewsHTTP(t *testing.T) {
	id := uuid.New()
	for _, tc := range []struct {
		name, path string
		err        error
		status     int
		called     bool
	}{
		{"empty", id.String(), nil, 200, true},
		{"unknown", id.String(), contract.ErrNotFound, 404, true},
		{"invalid", "invalid", nil, 404, false},
		{"nil", uuid.Nil.String(), nil, 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &topReader{err: tc.err}
			h := &Handler{listReviews: query.NewListReviewsHandler(reader)}
			e := echo.New()
			e.HTTPErrorHandler = httpserver.ErrorHandler
			Register(e.Group("/v1"), h, nil, nil)
			rec := httptest.NewRecorder()
			path := "/v1/media-objects/" + tc.path + "/top-reviews"
			if tc.name == "empty" {
				path = "/v1/media-objects/" + id.String() + "/top-reviews?limit=100"
			}
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != tc.status || reader.called != tc.called {
				t.Fatalf("status=%d called=%v body=%s", rec.Code, reader.called, rec.Body.String())
			}
			if tc.status == 200 && rec.Body.String() != "{\"items\":[]}\n" {
				t.Fatalf("body=%s", rec.Body.String())
			}
		})
	}
}

func TestReviewPublicReadAndProtectedWrites(t *testing.T) {
	id := uuid.New()
	owner := uuid.New()
	reader := &topReader{review: ports.Review{ID: id, AuthorID: owner, Status: "published", Title: "Published"}}
	h := &Handler{getReview: query.NewGetReviewHandler(reader)}
	e := echo.New()
	e.HTTPErrorHandler = httpserver.ErrorHandler
	Register(e.Group("/v1"), h, nil, nil)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/reviews/" + id.String(), 200}, {"POST", "/reviews", 401}, {"GET", "/profile/reviews", 401},
		{"PATCH", "/reviews/" + id.String(), 401}, {"DELETE", "/reviews/" + id.String(), 401},
		{"PUT", "/reviews/" + id.String() + "/vote", 401}, {"POST", "/reviews/" + id.String() + "/reports", 401},
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(tc.method, "/v1"+tc.path, nil))
		if rec.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	reader.review.Status = "draft"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/reviews/"+id.String(), nil))
	if rec.Code != 404 {
		t.Fatalf("draft exposed: %d %s", rec.Code, rec.Body.String())
	}
}
