package notificationshttp

import (
	"context"
	"errors"
	"net/http"

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
	list   *query.ListPrefsHandler
	update *command.UpdatePrefsHandler
	users  UserResolver
}

func NewHandler(list *query.ListPrefsHandler, update *command.UpdatePrefsHandler, users UserResolver) *Handler {
	return &Handler{list: list, update: update, users: users}
}

func (h *Handler) Get(c echo.Context) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	items, err := h.list.Handle(c.Request().Context(), userID)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, listResponse{Items: prefsJSON(items)})
}

func (h *Handler) Patch(c echo.Context) error {
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
	items, err := h.update.Handle(c.Request().Context(), userID, patches)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, listResponse{Items: prefsJSON(items)})
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

type prefJSON struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

type listResponse struct {
	Items []prefJSON `json:"items"`
}

type patchItem struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type patchBody struct {
	Items []patchItem `json:"items"`
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

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid notification settings",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
	default:
		return err
	}
}
