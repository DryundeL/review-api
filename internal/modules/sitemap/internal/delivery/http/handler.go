package sitemaphttp

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/sitemap/internal/application/query"
)

func Register(g *echo.Group, h *query.ListHandler) {
	g.GET("/sitemap/:entity", func(c echo.Context) error {
		req, err := query.ParseRequest(c.Param("entity"), c.QueryParam("limit"), c.QueryParam("cursor"))
		if err != nil {
			return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid sitemap request")
		}
		page, err := h.Handle(c.Request().Context(), req)
		if errors.Is(err, query.ErrInvalid) {
			return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid sitemap request")
		}
		if err != nil {
			return err
		}
		// Visibility can change between requests; do not cache private profile IDs.
		c.Response().Header().Set("Cache-Control", "no-store")
		return c.JSON(http.StatusOK, page)
	})
}
