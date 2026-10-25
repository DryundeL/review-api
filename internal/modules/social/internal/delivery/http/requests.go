package socialhttp

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/query"
)

type followStateJSON struct {
	FollowStatus string `json:"followStatus"`
	IsFollowing  bool   `json:"isFollowing"`
}

func (h *Handler) SetRequests(requests *query.FollowRequestsHandler) { h.requests = requests }

func (h *Handler) RequestFollow(c echo.Context) error {
	return h.followUser(c, c.Param("id"))
}

func (h *Handler) FollowStatus(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		return mapError(contract.ErrNotFound)
	}
	viewer, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	status, err := h.requests.Status(c.Request().Context(), id, viewer)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, followStateJSON{FollowStatus: status, IsFollowing: status == "following"})
}
func (h *Handler) ListFollowRequests(c echo.Context) error {
	owner, err := h.requireUser(c)
	if err != nil {
		return err
	}
	limit, offset, err := parsePage(c)
	if err != nil {
		return mapError(err)
	}
	items, err := h.requests.List(c.Request().Context(), owner, limit, offset)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, listResponse{Items: followItems(items)})
}
func (h *Handler) ApproveFollowRequest(c echo.Context) error { return h.resolveRequest(c, true) }
func (h *Handler) RejectFollowRequest(c echo.Context) error  { return h.resolveRequest(c, false) }
func (h *Handler) resolveRequest(c echo.Context, approve bool) error {
	owner, err := h.requireUser(c)
	if err != nil {
		return err
	}
	follower, err := uuid.Parse(c.Param("userId"))
	if err != nil || follower == uuid.Nil {
		return mapError(contract.FieldError{Field: "userId"})
	}
	if approve {
		err = h.follow.Approve(c.Request().Context(), follower, owner)
	} else {
		err = h.follow.Reject(c.Request().Context(), follower, owner)
	}
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}
