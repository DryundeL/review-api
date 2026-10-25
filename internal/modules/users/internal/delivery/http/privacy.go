package usershttp

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"review-api/internal/http/httperr"
	"review-api/internal/modules/users/contract"
)

// ProfileAccess runs after optional authentication on profile subresources.
func (h *Handler) ProfileAccess(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		id, err := uuid.Parse(c.Param("id"))
		if err != nil || id == uuid.Nil {
			return httperr.New(http.StatusNotFound, "NOT_FOUND", "user not found")
		}
		viewer, err := h.optionalViewer(c)
		if err != nil {
			return err
		}
		_, err = h.profile.ByID(c.Request().Context(), id, viewer)
		if errors.Is(err, contract.ErrNotFound) {
			return httperr.New(http.StatusNotFound, "NOT_FOUND", "user not found")
		}
		if err != nil {
			return err
		}
		return next(c)
	}
}
