package usershttp

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
	"review-api/internal/http/httperr"
	"review-api/internal/modules/users/internal/application/command"
)

func (h *Handler) SetContentPreferences(content *command.ContentPreferencesHandler) {
	h.content = content
}

func (h *Handler) GetContentPreferences(c echo.Context) error {
	id, err := h.requireUser(c)
	if err != nil {
		return err
	}
	out, err := h.content.Get(c.Request().Context(), id)
	if err != nil {
		return mapSettingsError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) PutContentPreferences(c echo.Context) error {
	id, err := h.requireUser(c)
	if err != nil {
		return err
	}
	var body struct {
		Types []string `json:"types"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response().Writer, c.Request().Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return httperr.New(400, "VALIDATION_ERROR", "invalid request body")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return httperr.New(400, "VALIDATION_ERROR", "invalid request body")
	}
	out, err := h.content.Save(c.Request().Context(), id, body.Types)
	if err != nil {
		return mapSettingsError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, out)
}
