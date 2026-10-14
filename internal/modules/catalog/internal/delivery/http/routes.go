package cataloghttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, auth echo.MiddlewareFunc) {
	g.GET("/works", h.Search)
	g.GET("/works/:id", h.Get)
	if auth == nil {
		auth = func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(echo.Context) error {
				return httperr.Unauthorized("unauthorized")
			}
		}
	}
	g.POST("/works", h.Create, auth)
}
