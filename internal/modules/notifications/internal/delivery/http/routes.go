package notificationshttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, auth echo.MiddlewareFunc) {
	if auth == nil {
		auth = func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(echo.Context) error {
				return httperr.Unauthorized("unauthorized")
			}
		}
	}
	g.GET("/notification-settings", h.GetSettings, auth)
	g.PATCH("/notification-settings", h.PatchSettings, auth)

	g.GET("/notifications", h.List, auth)
	g.GET("/notifications/unread", h.Unread, auth)
	g.PATCH("/notifications/read", h.MarkAllRead, auth)
	g.PATCH("/notifications/:id/read", h.MarkRead, auth)
}
