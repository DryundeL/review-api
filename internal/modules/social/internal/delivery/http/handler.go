package socialhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/command"
	"review-api/internal/modules/social/internal/application/ports"
	"review-api/internal/modules/social/internal/application/query"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	follow        *command.FollowHandler
	unfollow      *command.UnfollowHandler
	listFollowing *query.ListFollowingHandler
	listFollowers *query.ListFollowersHandler
	users         UserResolver
}

func NewHandler(
	follow *command.FollowHandler,
	unfollow *command.UnfollowHandler,
	listFollowing *query.ListFollowingHandler,
	listFollowers *query.ListFollowersHandler,
	users UserResolver,
) *Handler {
	return &Handler{
		follow: follow, unfollow: unfollow,
		listFollowing: listFollowing, listFollowers: listFollowers,
		users: users,
	}
}

func (h *Handler) Follow(c echo.Context) error {
	followerID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	followeeID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return mapError(contract.FieldError{Field: "followeeId"})
	}
	if err := h.follow.Handle(c.Request().Context(), followerID, followeeID); err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) Unfollow(c echo.Context) error {
	followerID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	followeeID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return mapError(contract.FieldError{Field: "followeeId"})
	}
	if err := h.unfollow.Handle(c.Request().Context(), followerID, followeeID); err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListFollowing(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	items, err := h.listFollowing.Handle(c.Request().Context(), userID, parseLimit(c))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, listResponse{Items: followItems(items, true)})
}

func (h *Handler) ListFollowers(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	items, err := h.listFollowers.Handle(c.Request().Context(), userID, parseLimit(c))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, listResponse{Items: followItems(items, false)})
}

func (h *Handler) resolveUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

func parseLimit(c echo.Context) int {
	raw := c.QueryParam("limit")
	if raw == "" {
		return 20
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 20
	}
	return n
}

type followJSON struct {
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
}

type listResponse struct {
	Items []followJSON `json:"items"`
}

func followItems(items []ports.Follow, following bool) []followJSON {
	out := make([]followJSON, 0, len(items))
	for _, item := range items {
		uid := item.FolloweeID
		if !following {
			uid = item.FollowerID
		}
		out = append(out, followJSON{UserID: uid.String(), CreatedAt: item.CreatedAt})
	}
	return out
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid follow",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "follow not found")
	case errors.Is(err, contract.ErrConflict):
		return httperr.New(http.StatusConflict, "CONFLICT", "already following")
	case errors.Is(err, contract.ErrForbidden):
		return httperr.New(http.StatusForbidden, "FORBIDDEN", "forbidden")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid follow")
	default:
		return err
	}
}
