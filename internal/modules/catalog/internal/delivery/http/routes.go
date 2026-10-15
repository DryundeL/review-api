package cataloghttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, admin echo.MiddlewareFunc) {
	g.GET("/media-objects", h.Search)
	g.GET("/media-objects/:id", h.Get)
	if admin == nil {
		admin = func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(echo.Context) error {
				return httperr.Unauthorized("unauthorized")
			}
		}
	}
	g.POST("/admin/media-objects/import", h.Import, admin)
}
