package gamificationhttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, admin, auth echo.MiddlewareFunc) {
	if auth == nil {
		auth = denyUnauthorized
	}
	if admin == nil {
		admin = denyUnauthorized
	}

	g.GET("/shop/items", h.ListShop)
	g.GET("/gamification/me", h.Me, auth)
	g.POST("/shop/purchases", h.Purchase, auth)
	g.PUT("/gamification/equipped", h.Equip, auth)

	adminGroup := g.Group("/admin/shop-items", admin)
	adminGroup.GET("", h.AdminList)
	adminGroup.POST("", h.CreateItem)
	adminGroup.PATCH("/:id", h.UpdateItem)
}

func denyUnauthorized(next echo.HandlerFunc) echo.HandlerFunc {
	return func(echo.Context) error {
		return httperr.Unauthorized("unauthorized")
	}
}
