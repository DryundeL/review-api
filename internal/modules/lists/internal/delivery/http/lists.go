package listshttp

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/lists/internal/application/query"
)

func listInput(c echo.Context) (query.ListInput, error) {
	limit, err := parseLimit(c)
	if err != nil {
		return query.ListInput{}, err
	}
	return query.ListInput{
		Search: c.QueryParam("q"), Type: c.QueryParam("type"), Format: c.QueryParam("format"),
		Cursor: c.QueryParam("cursor"), Limit: limit,
	}, nil
}

func mineInput(c echo.Context) (query.MineInput, error) {
	limit, err := parseLimit(c)
	if err != nil {
		return query.MineInput{}, err
	}
	return query.MineInput{Status: c.QueryParam("status"), Cursor: c.QueryParam("cursor"), Limit: limit}, nil
}

type publicList func(context.Context, query.ListInput) (query.ListPage, error)

type profileList func(context.Context, uuid.UUID, query.ListInput) (query.ListPage, error)

type mineList func(context.Context, uuid.UUID, query.MineInput) (query.ListPage, error)

// Public lists are the same for everyone, so they carry no viewer-specific fields.
func (h *Handler) public(c echo.Context, list publicList) error {
	in, err := listInput(c)
	if err != nil {
		return err
	}
	page, err := list(c.Request().Context(), in)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, pageResponse(page, false))
}

// profile runs after profileAccess, which already returned 404 for a closed or missing profile.
func (h *Handler) profile(c echo.Context, list profileList) error {
	authorID, err := pathID(c, "id")
	if err != nil {
		return err
	}
	in, err := listInput(c)
	if err != nil {
		return err
	}
	page, err := list(c.Request().Context(), authorID, in)
	if err != nil {
		return mapError(err)
	}
	private(c, false)
	return c.JSON(http.StatusOK, pageResponse(page, false))
}

func (h *Handler) mine(c echo.Context, list mineList) error {
	userID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	in, err := mineInput(c)
	if err != nil {
		return err
	}
	page, err := list(c.Request().Context(), userID, in)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(http.StatusOK, pageResponse(page, true))
}

func (h *Handler) ListTemplates(c echo.Context) error { return h.public(c, h.d.Lists.Templates) }

func (h *Handler) ListResults(c echo.Context) error { return h.public(c, h.d.Lists.Results) }

func (h *Handler) ProfileTemplates(c echo.Context) error {
	return h.profile(c, h.d.Lists.ProfileTemplates)
}

func (h *Handler) ProfileResults(c echo.Context) error { return h.profile(c, h.d.Lists.ProfileResults) }

func (h *Handler) MyTemplates(c echo.Context) error { return h.mine(c, h.d.Lists.MyTemplates) }

func (h *Handler) MyResults(c echo.Context) error { return h.mine(c, h.d.Lists.MyResults) }
