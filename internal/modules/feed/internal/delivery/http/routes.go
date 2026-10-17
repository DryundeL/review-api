package feedhttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/feed/internal/application/ports"
)

func Register(g *echo.Group, h *Handler, auth echo.MiddlewareFunc) {
	g.GET("/feed", h.ListFeed, feedAuth(auth))
}

// feedAuth: Bearer optional for isFollowing; required when mode=following.
func feedAuth(auth echo.MiddlewareFunc) echo.MiddlewareFunc {
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
			if c.Request().Header.Get(echo.HeaderAuthorization) == "" {
				return next(c)
			}
			return auth(next)(c)
		}
	}
}
