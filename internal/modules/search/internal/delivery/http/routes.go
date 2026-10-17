package searchhttp

import (
	"github.com/labstack/echo/v4"
)

func Register(g *echo.Group, h *Handler, auth echo.MiddlewareFunc) {
	opt := optionalAuth(auth)
	g.GET("/search/trends", h.Trends, opt)
	g.GET("/search", h.Search, opt)
}

// optionalAuth: no Bearer → anonymous; with Bearer → validate for isFollowing.
func optionalAuth(auth echo.MiddlewareFunc) echo.MiddlewareFunc {
	if auth == nil {
		auth = func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				return echo.ErrUnauthorized
			}
		}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get(echo.HeaderAuthorization) == "" {
				return next(c)
			}
			return auth(next)(c)
		}
	}
}
