package reviewshttp

import (
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
)

func Register(g *echo.Group, h *Handler, admin, auth echo.MiddlewareFunc, profileAccess ...echo.MiddlewareFunc) {
	g.GET("/review-types", h.List)
	g.GET("/review-statuses", h.Statuses)
	g.GET("/report-reasons", h.ListReportReasons)

	if auth == nil {
		auth = denyUnauthorized
	}
	if admin == nil {
		admin = denyUnauthorized
	}
	opt := optionalAuth(auth)
	profileMiddleware := append([]echo.MiddlewareFunc{opt}, profileAccess...)

	g.GET("/profile/reviews", h.ListMyReviews, auth)
	g.GET("/profiles/:id/reviews", h.ListUserReviews, profileMiddleware...)

	g.GET("/media-objects/:id/top-reviews", h.TopReviews, opt)
	g.GET("/reviews", h.ListReviews, opt)
	g.GET("/reviews/:id", h.GetReview, opt)
	g.POST("/reviews", h.CreateReview, auth)
	g.PATCH("/reviews/:id", h.UpdateReview, auth)
	g.DELETE("/reviews/:id", h.DeleteReview, auth)
	g.PUT("/reviews/:id/vote", h.VoteReview, auth)
	g.PUT("/reviews/:id/view", h.ViewReview, auth)
	g.POST("/reviews/:id/reports", h.CreateReport, auth)

	adminGroup := g.Group("/admin/review-types", admin)
	adminGroup.GET("", h.AdminList)
	adminGroup.POST("", h.Create)
	adminGroup.PATCH("/:code", h.Update)
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

func denyUnauthorized(next echo.HandlerFunc) echo.HandlerFunc {
	return func(echo.Context) error {
		return httperr.Unauthorized("unauthorized")
	}
}
