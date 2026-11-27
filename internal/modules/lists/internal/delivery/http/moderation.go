package listshttp

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"review-api/internal/modules/lists/internal/application/ports"
)

type reportBody struct {
	ReasonCode string `json:"reasonCode"`
}

func (h *Handler) ReportTemplate(c echo.Context) error { return h.report(c, ports.TargetTemplate) }

func (h *Handler) ReportResult(c echo.Context) error { return h.report(c, ports.TargetResult) }

func (h *Handler) report(c echo.Context, target ports.ReportTarget) error {
	userID, id, err := h.userAndID(c)
	if err != nil {
		return err
	}
	var body reportBody
	if err := bind(c, &body); err != nil {
		return err
	}
	if err := h.allowReport(c, userID); err != nil {
		return err
	}
	rep, err := h.d.Reports.Create(c.Request().Context(), target, id, userID, body.ReasonCode)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(http.StatusCreated, reportResponse(rep))
}

func (h *Handler) ReportQueue(c echo.Context) error {
	limit, err := parseLimit(c)
	if err != nil {
		return err
	}
	page, err := h.d.ReportQueue.Handle(c.Request().Context(), c.QueryParam("status"), c.QueryParam("cursor"), limit)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(http.StatusOK, reportPageResponse(page))
}

func (h *Handler) ResolveReport(c echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	rep, err := h.d.Moderation.Resolve(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	noStore(c)
	return c.JSON(http.StatusOK, reportResponse(rep))
}

func (h *Handler) HideTemplate(c echo.Context) error {
	return h.setHidden(c, ports.TargetTemplate, true)
}

func (h *Handler) UnhideTemplate(c echo.Context) error {
	return h.setHidden(c, ports.TargetTemplate, false)
}

func (h *Handler) HideResult(c echo.Context) error { return h.setHidden(c, ports.TargetResult, true) }

func (h *Handler) UnhideResult(c echo.Context) error {
	return h.setHidden(c, ports.TargetResult, false)
}

func (h *Handler) setHidden(c echo.Context, target ports.ReportTarget, hidden bool) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if hidden {
		err = h.d.Moderation.Hide(ctx, target, id)
	} else {
		err = h.d.Moderation.Unhide(ctx, target, id)
	}
	if err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
