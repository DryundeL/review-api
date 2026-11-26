package listshttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/lists/contract"
)

func TestMapError(t *testing.T) {
	t.Parallel()
	for err, want := range map[error]int{
		contract.ErrNotFound:         http.StatusNotFound,
		contract.ErrRevisionConflict: http.StatusConflict,
		contract.ErrAlreadyReported:  http.StatusConflict,
		contract.ErrConflict:         http.StatusConflict,
		contract.ErrModerated:        http.StatusForbidden,
		contract.ErrForbidden:        http.StatusForbidden,
		contract.ErrQuotaExceeded:    http.StatusUnprocessableEntity,
		contract.ErrInvalid:          http.StatusUnprocessableEntity,
		fmt.Errorf("wrapped: %w", contract.FieldAt("items", 3, contract.ReasonDuplicate)): http.StatusUnprocessableEntity,
	} {
		he, ok := errors.AsType[*httperr.Error](mapError(err))
		if !ok || he.Status != want {
			t.Fatalf("%v -> %+v, want %d", err, he, want)
		}
	}
	he, _ := errors.AsType[*httperr.Error](mapError(contract.FieldAt("items", 3, contract.ReasonDuplicate)))
	if d, ok := he.Details.(map[string]any); !ok || d["field"] != "items" || d["index"] != 3 || d["reason"] != "duplicate" {
		t.Fatalf("details = %#v", he.Details)
	}
	he, _ = errors.AsType[*httperr.Error](mapError(contract.Field("title", contract.ReasonRequired)))
	if d := he.Details.(map[string]any); d["index"] != nil {
		t.Fatalf("index without a collection: %#v", d)
	}
	plain := errors.New("db down")
	if got := mapError(plain); got != plain {
		t.Fatalf("unknown errors must pass through to the 500 handler: %v", got)
	}
}

type fakeLimiter struct {
	ok  bool
	err error
}

func (l fakeLimiter) Allow(context.Context, uuid.UUID) (bool, error) { return l.ok, l.err }

func TestAllowReport(t *testing.T) {
	t.Parallel()
	ctx := func() echo.Context {
		return echo.New().NewContext(httptest.NewRequest(http.MethodPost, "/", nil), httptest.NewRecorder())
	}
	user := uuid.New()
	if err := NewHandler(Deps{}).allowReport(ctx(), user); err != nil {
		t.Fatalf("no limiter: %v", err)
	}
	if err := NewHandler(Deps{ReportLimiter: fakeLimiter{ok: true}}).allowReport(ctx(), user); err != nil {
		t.Fatalf("allowed: %v", err)
	}
	err := NewHandler(Deps{ReportLimiter: fakeLimiter{}}).allowReport(ctx(), user)
	if he, ok := errors.AsType[*httperr.Error](err); !ok || he.Status != http.StatusTooManyRequests {
		t.Fatalf("over the limit: %v", err)
	}
	if err := NewHandler(Deps{ReportLimiter: fakeLimiter{err: errors.New("redis down")}}).allowReport(ctx(), user); err != nil {
		t.Fatalf("limiter errors must fail open: %v", err)
	}
}
