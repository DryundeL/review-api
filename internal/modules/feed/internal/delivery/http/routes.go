package feedhttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/feed/internal/application/ports"
)

func Register(g *echo.Group, h *Handler, auth echo.MiddlewareFunc) {
	g.GET("/feed", h.ListFeed, requireAuthForFollowing(auth))
}

// requireAuthForFollowing runs Bearer only when mode=following.
func requireAuthForFollowing(auth echo.MiddlewareFunc) echo.MiddlewareFunc {
	if auth == nil {
		auth = func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return echo.ErrUnauthorized
			}
		}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.QueryParam("mode") == ports.ModeFollowing {
				return auth(next)(c)
			}
			return next(c)
		}
	}
}
