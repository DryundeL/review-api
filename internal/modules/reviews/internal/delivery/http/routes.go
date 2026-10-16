package reviewshttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, admin, auth echo.MiddlewareFunc) {
	g.GET("/review-types", h.List)
	g.GET("/review-statuses", h.Statuses)

	if auth == nil {
		auth = denyUnauthorized
	}
	if admin == nil {
		admin = denyUnauthorized
	}

	g.GET("/profile/reviews", h.ListMyReviews, auth)
	g.GET("/profiles/:id/reviews", h.ListUserReviews)

	g.GET("/reviews", h.ListReviews)
	g.GET("/reviews/:id", h.GetReview, auth)
	g.POST("/reviews", h.CreateReview, auth)
	g.PATCH("/reviews/:id", h.UpdateReview, auth)
	g.DELETE("/reviews/:id", h.DeleteReview, auth)
	g.PUT("/reviews/:id/vote", h.VoteReview, auth)
	g.PUT("/reviews/:id/view", h.ViewReview, auth)

	adminGroup := g.Group("/admin/review-types", admin)
	adminGroup.GET("", h.AdminList)
	adminGroup.POST("", h.Create)
	adminGroup.PATCH("/:code", h.Update)
}

func denyUnauthorized(next echo.HandlerFunc) echo.HandlerFunc {
	return func(echo.Context) error {
		return httperr.Unauthorized("unauthorized")
	}
}
