package mediahttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, auth echo.MiddlewareFunc) {
	if auth == nil {
		auth = func(echo.HandlerFunc) echo.HandlerFunc {
			return func(echo.Context) error { return httperr.Unauthorized("unauthorized") }
		}
	}
	g.POST("/media/images", h.UploadImage, auth)
}
