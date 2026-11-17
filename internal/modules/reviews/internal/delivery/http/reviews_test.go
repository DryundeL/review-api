package reviewshttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/reviews/contract"
)

func TestMapReviewErrorReason(t *testing.T) {
	var httpErr *httperr.Error
	err := mapReviewError(contract.FieldError{Field: "body", Reason: "too_long"})
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("want 422: %v", err)
	}
	details, _ := httpErr.Details.(map[string]string)
	if details["field"] != "body" || details["reason"] != "too_long" {
		t.Fatalf("details=%v", httpErr.Details)
	}
	if !errors.As(mapReviewError(contract.ErrRevisionConflict), &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Code != "REVISION_CONFLICT" {
		t.Fatalf("want 409 REVISION_CONFLICT: %+v", httpErr)
	}
}

func TestBodyLimitReturns413(t *testing.T) {
	e := echo.New()
	handler := middleware.BodyLimit(maxWriteBodyBytes)(func(c echo.Context) error {
		var body saveDraftBody
		if err := c.Bind(&body); err != nil {
			return bindError(err)
		}
		return c.NoContent(http.StatusOK)
	})
	payload := `{"body":"` + strings.Repeat("a", 300*1024) + `","revision":1}`
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	err := handler(e.NewContext(req, httptest.NewRecorder()))
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %v", err)
	}
}
