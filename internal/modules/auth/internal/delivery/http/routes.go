package authhttp

import "github.com/labstack/echo/v4"

func RegisterRoutes(g *echo.Group, h *Handler) {
	g.POST("/auth/telegram", h.Telegram)
	g.POST("/auth/login", h.StartLogin)
	g.POST("/auth/login/poll", h.PollLogin)
	g.GET("/auth/identity", h.Identity, h.Middleware)
}
