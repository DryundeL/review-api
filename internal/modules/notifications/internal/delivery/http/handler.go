package notificationshttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/notifications/contract"
	"review-api/internal/modules/notifications/internal/application/command"
	"review-api/internal/modules/notifications/internal/application/ports"
	"review-api/internal/modules/notifications/internal/application/query"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	listPrefs   *query.ListPrefsHandler
	updatePrefs *command.UpdatePrefsHandler
	listInbox   *query.ListInboxHandler
	unread      *query.UnreadCountHandler
	markRead    *command.MarkReadHandler
	markAllRead *command.MarkAllReadHandler
	users       UserResolver
}

func NewHandler(
	listPrefs *query.ListPrefsHandler,
	updatePrefs *command.UpdatePrefsHandler,
	listInbox *query.ListInboxHandler,
	unread *query.UnreadCountHandler,
	markRead *command.MarkReadHandler,
	markAllRead *command.MarkAllReadHandler,
	users UserResolver,
) *Handler {
	return &Handler{
		listPrefs: listPrefs, updatePrefs: updatePrefs,
		listInbox: listInbox, unread: unread,
		markRead: markRead, markAllRead: markAllRead,
		users: users,
	}
}

func (h *Handler) GetSettings(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	items, err := h.listPrefs.Handle(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, prefsListResponse{Items: prefsJSON(items)})
}

func (h *Handler) PatchSettings(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	var body patchBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	patches := make([]ports.PrefPatch, 0, len(body.Items))
	for _, item := range body.Items {
		patches = append(patches, ports.PrefPatch{Name: item.Name, Enabled: item.Enabled})
	}
	items, err := h.updatePrefs.Handle(c.Request().Context(), userID, patches)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, prefsListResponse{Items: prefsJSON(items)})
}

func (h *Handler) List(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	limit, offset, err := parsePage(c)
	if err != nil {
		return mapError(err)
	}
	items, err := h.listInbox.Handle(c.Request().Context(), userID, limit, offset)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, inboxListResponse{Items: inboxJSON(items)})
}

func (h *Handler) Unread(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	n, err := h.unread.Handle(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, unreadResponse{Count: n})
}

func (h *Handler) MarkRead(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	}
	if err := h.markRead.Handle(c.Request().Context(), userID, id); err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) MarkAllRead(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	if err := h.markAllRead.Handle(c.Request().Context(), userID); err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) requireUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	if h.users == nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

func parsePage(c echo.Context) (limit, offset int, err error) {
	limit = 20
	if raw := c.QueryParam("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return 0, 0, contract.FieldError{Field: "limit"}
		}
	}
	if raw := c.QueryParam("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, contract.FieldError{Field: "offset"}
		}
	}
	return limit, offset, nil
}

type prefJSON struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

type prefsListResponse struct {
	Items []prefJSON `json:"items"`
}

type patchItem struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type patchBody struct {
	Items []patchItem `json:"items"`
}

type inboxItemJSON struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	ActorID   *string `json:"actorId,omitempty"`
	ReviewID  *string `json:"reviewId,omitempty"`
	IsRead    bool    `json:"isRead"`
	CreatedAt string  `json:"createdAt"`
}

type inboxListResponse struct {
	Items []inboxItemJSON `json:"items"`
}

type unreadResponse struct {
	Count int64 `json:"count"`
}

func prefsJSON(items []ports.PrefItem) []prefJSON {
	out := make([]prefJSON, 0, len(items))
	for _, item := range items {
		out = append(out, prefJSON{
			Name: item.Name, Title: item.Title, Text: item.Text, Enabled: item.Enabled,
		})
	}
	return out
}

func inboxJSON(items []ports.Notification) []inboxItemJSON {
	out := make([]inboxItemJSON, 0, len(items))
	for _, item := range items {
		row := inboxItemJSON{
			ID:        item.ID.String(),
			Type:      item.TypeName,
			Title:     item.Title,
			Body:      item.Body,
			IsRead:    item.ReadAt != nil,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		}
		if item.ActorID != nil {
			s := item.ActorID.String()
			row.ActorID = &s
		}
		if item.ReviewID != nil {
			s := item.ReviewID.String()
			row.ReviewID = &s
		}
		out = append(out, row)
	}
	return out
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid notification request",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	default:
		return err
	}
}
