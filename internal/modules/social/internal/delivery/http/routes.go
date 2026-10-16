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
	g.GET("/follows/following", h.ListFollowing, auth)
	g.GET("/follows/followers", h.ListFollowers, auth)
	g.PUT("/follows/:userId", h.Follow, auth)
	g.DELETE("/follows/:userId", h.Unfollow, auth)
}
