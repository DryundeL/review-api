package listshttp

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"review-api/internal/http/httperr"
)

// maxWriteBodyBytes caps JSON writes: 50 items and 10 tiers with full-length text fit well below it.
const maxWriteBodyBytes = "128K"

func Register(g *echo.Group, h *Handler, admin, auth echo.MiddlewareFunc, profileAccess ...echo.MiddlewareFunc) {
	if auth == nil {
		auth = denyUnauthorized
	}
	if admin == nil {
		admin = denyUnauthorized
	}
	opt := optionalAuth(auth)
	profile := append([]echo.MiddlewareFunc{opt}, profileAccess...)
	limit := middleware.BodyLimit(maxWriteBodyBytes)

	g.GET("/list-templates", h.ListTemplates, opt)
	g.GET("/list-results", h.ListResults, opt)
	g.GET("/profiles/:id/list-templates", h.ProfileTemplates, profile...)
	g.GET("/profiles/:id/list-results", h.ProfileResults, profile...)
	g.GET("/profile/list-templates", h.MyTemplates, auth)
	g.GET("/profile/list-results", h.MyResults, auth)

	g.POST("/list-templates", h.CreateTemplate, auth, limit)
	g.GET("/list-templates/:id", h.GetTemplate, opt)
	g.PUT("/list-templates/:id", h.SaveTemplate, auth, limit)
	g.DELETE("/list-templates/:id", h.DeleteTemplate, auth)
	g.POST("/list-templates/:id/publish", h.PublishTemplate, auth, limit)
	g.POST("/list-templates/:id/unpublish", h.UnpublishTemplate, auth)
	g.PATCH("/list-templates/:id/visibility", h.TemplateVisibility, auth, limit)
	g.POST("/list-templates/:id/copy", h.CopyTemplate, auth)
	g.POST("/list-templates/:id/items/:itemId/refresh-from-catalog", h.RefreshItem, auth, limit)
	g.POST("/list-templates/:id/results", h.CreateResult, auth)
	g.POST("/list-templates/:id/reports", h.ReportTemplate, auth, limit)

	g.GET("/list-results/:id", h.GetResult, opt)
	g.PUT("/list-results/:id", h.SaveResult, auth, limit)
	g.DELETE("/list-results/:id", h.DeleteResult, auth)
	g.POST("/list-results/:id/publish", h.PublishResult, auth, limit)
	g.POST("/list-results/:id/unpublish", h.UnpublishResult, auth)
	g.PATCH("/list-results/:id/visibility", h.ResultVisibility, auth, limit)
	g.POST("/list-results/:id/copy", h.CopyResult, auth)
	g.POST("/list-results/:id/reports", h.ReportResult, auth, limit)
	g.PUT("/list-results/:id/vote", h.VoteResult, auth, limit)
	g.PUT("/list-results/:id/view", h.ViewResult, auth)
	g.GET("/list-results/:id/comments", h.ListResultComments, opt)
	g.POST("/list-results/:id/comments", h.CreateResultComment, auth)

	// Admin middleware per route: a "/admin" group would add a catch-all shared with other modules.
	g.GET("/admin/list-reports", h.ReportQueue, admin)
	g.POST("/admin/list-reports/:id/resolve", h.ResolveReport, admin)
	g.POST("/admin/list-templates/:id/hide", h.HideTemplate, admin)
	g.POST("/admin/list-templates/:id/unhide", h.UnhideTemplate, admin)
	g.POST("/admin/list-results/:id/hide", h.HideResult, admin)
	g.POST("/admin/list-results/:id/unhide", h.UnhideResult, admin)
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

func denyUnauthorized(echo.HandlerFunc) echo.HandlerFunc {
	return func(echo.Context) error {
		return httperr.Unauthorized("unauthorized")
	}
}
