package socialhttp

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
	opt := optionalAuth(auth)
	// Nested under profiles — same resource as GET /profiles/:id (not /users).
	g.GET("/profiles/:id/following", h.ListFollowing, opt)
	g.GET("/profiles/:id/followers", h.ListFollowers, opt)
	g.PUT("/follows/:userId", h.Follow, auth)
	g.DELETE("/follows/:userId", h.Unfollow, auth)
}

func optionalAuth(auth echo.MiddlewareFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get(echo.HeaderAuthorization) == "" {
				return next(c)
			}
			return auth(next)(c)
		}
	}
}
