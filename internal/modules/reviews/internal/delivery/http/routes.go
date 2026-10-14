package reviewshttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, admin echo.MiddlewareFunc) {
	g.GET("/review-types", h.List)
	g.GET("/review-statuses", h.Statuses)
	if admin == nil {
		admin = func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(echo.Context) error {
				return httperr.Unauthorized("unauthorized")
			}
		}
	}
	adminGroup := g.Group("/admin/review-types", admin)
	adminGroup.GET("", h.AdminList)
	adminGroup.POST("", h.Create)
	adminGroup.PATCH("/:code", h.Update)
}
