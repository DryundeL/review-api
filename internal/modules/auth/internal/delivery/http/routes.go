package authhttp

import "github.com/labstack/echo/v4"

func RegisterRoutes(g *echo.Group, h *Handler) {
	g.GET("/auth/identity", h.Identity, h.Middleware)
}
