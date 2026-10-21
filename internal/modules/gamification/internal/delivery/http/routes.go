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
	opt := optionalAuth(auth)

	g.GET("/shop/items", h.ListShop)
	g.GET("/gamification/me", h.Me, auth)
	g.POST("/shop/purchases", h.Purchase, auth)
	g.PUT("/gamification/equipped", h.Equip, auth)

	g.GET("/profiles/:id/achievements", h.ListProfileAchievements, opt)

	adminShop := g.Group("/admin/shop-items", admin)
	adminShop.GET("", h.AdminList)
	adminShop.POST("", h.CreateItem)
	adminShop.PATCH("/:id", h.UpdateItem)

	adminAchievements := g.Group("/admin/achievements", admin)
	adminAchievements.GET("", h.AdminListAchievements)
	adminAchievements.POST("", h.CreateAchievement)
	adminAchievements.PATCH("/:id", h.UpdateAchievement)
}

func denyUnauthorized(next echo.HandlerFunc) echo.HandlerFunc {
	return func(echo.Context) error {
		return httperr.Unauthorized("unauthorized")
	}
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
