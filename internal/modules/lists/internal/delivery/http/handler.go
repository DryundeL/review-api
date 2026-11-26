package listshttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/command"
	"review-api/internal/modules/lists/internal/application/query"
)

// UserResolver maps the Telegram identity to the internal user id.
type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

// ReportLimiter counts report attempts per user. A false result means the limit is reached.
type ReportLimiter interface {
	Allow(ctx context.Context, userID uuid.UUID) (bool, error)
}

type Deps struct {
	CreateTemplate *command.CreateTemplateHandler
	SaveTemplate   *command.SaveTemplateHandler
	TemplateState  *command.TemplateStateHandler
	CopyTemplate   *command.CopyTemplateHandler
	GetTemplate    *query.GetTemplateHandler
	Results        *command.ResultHandler
	GetResult      *query.GetResultHandler
	Lists          *query.ListHandler
	Reports        *command.ReportHandler
	Moderation     *command.ModerationHandler
	ReportQueue    *query.ReportQueueHandler
	Users          UserResolver
	ReportLimiter  ReportLimiter
}

type Handler struct {
	d Deps
}

func NewHandler(d Deps) *Handler {
	return &Handler{d: d}
}

func (h *Handler) requireUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil || h.d.Users == nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	userID, err := h.d.Users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

// optionalUser is uuid.Nil for guests. A present but unknown identity is 401, as in reviews.
func (h *Handler) optionalUser(c echo.Context) (uuid.UUID, error) {
	if _, err := authcontract.IdentityFromContext(c.Request().Context()); err != nil {
		return uuid.Nil, nil
	}
	return h.requireUser(c)
}

// pathID parses a path UUID; a malformed one is the same 404 as a missing object.
func pathID(c echo.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

func bind(c echo.Context, dst any) error {
	if err := c.Bind(dst); err != nil {
		// Keep the 413 from the body limit instead of masking it as 400.
		if he, ok := errors.AsType[*echo.HTTPError](err); ok && he.Code == http.StatusRequestEntityTooLarge {
			return httperr.New(http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body is too large")
		}
		return httperr.New(http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}
	return nil
}

func requireRevision(revision int) error {
	if revision < 1 {
		return mapError(contract.Field("revision", contract.ReasonRequired))
	}
	return nil
}

// parseLimit leaves range checks to the query handlers; non-numbers are a field error too.
func parseLimit(c echo.Context) (int, error) {
	raw := c.QueryParam("limit")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, mapError(contract.Field("limit", contract.ReasonInvalid))
	}
	return n, nil
}

func noStore(c echo.Context) {
	c.Response().Header().Set("Cache-Control", "no-store")
}

// private marks responses that depend on the viewer: the author gets more fields.
func private(c echo.Context, isOwner bool) {
	c.Response().Header().Add("Vary", echo.HeaderAuthorization)
	if isOwner || c.Request().Header.Get(echo.HeaderAuthorization) != "" {
		noStore(c)
	}
}

func notFound() error {
	return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
}

func mapError(err error) error {
	if field, ok := errors.AsType[contract.FieldError](err); ok {
		details := map[string]any{"field": field.Field, "reason": field.Reason}
		if field.Index >= 0 {
			details["index"] = field.Index
		}
		return &httperr.Error{
			Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: "invalid input", Details: details,
		}
	}
	switch {
	case errors.Is(err, contract.ErrNotFound):
		return notFound()
	case errors.Is(err, contract.ErrRevisionConflict):
		return httperr.New(http.StatusConflict, "REVISION_CONFLICT", "changed elsewhere, reload and retry")
	case errors.Is(err, contract.ErrAlreadyReported):
		return httperr.New(http.StatusConflict, "ALREADY_REPORTED", "already reported")
	case errors.Is(err, contract.ErrConflict):
		return httperr.New(http.StatusConflict, "CONFLICT", "not allowed in the current state")
	case errors.Is(err, contract.ErrModerated):
		return httperr.New(http.StatusForbidden, "HIDDEN_BY_MODERATOR", "hidden by a moderator")
	case errors.Is(err, contract.ErrForbidden):
		return httperr.New(http.StatusForbidden, "FORBIDDEN", "forbidden")
	case errors.Is(err, contract.ErrQuotaExceeded):
		return httperr.New(http.StatusUnprocessableEntity, "QUOTA_EXCEEDED", "limit of lists reached")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid input")
	default:
		return err
	}
}

// allowReport fails open on limiter errors: the global per-IP limit still applies.
func (h *Handler) allowReport(c echo.Context, userID uuid.UUID) error {
	if h.d.ReportLimiter == nil {
		return nil
	}
	ok, err := h.d.ReportLimiter.Allow(c.Request().Context(), userID)
	if err != nil {
		slog.WarnContext(c.Request().Context(), "lists report limiter unavailable", "error", err)
		return nil
	}
	if !ok {
		return httperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many reports")
	}
	return nil
}
