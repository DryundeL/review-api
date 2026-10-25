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
	requests      *query.FollowRequestsHandler
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
	return h.followUser(c, c.Param("userId"))
}

func (h *Handler) followUser(c echo.Context, rawID string) error {
	followerID, err := h.requireUser(c)
	if err != nil {
		return err
	}
	followeeID, err := uuid.Parse(rawID)
	if err != nil {
		return mapError(contract.FieldError{Field: "followeeId"})
	}
	status, err := h.follow.Handle(c.Request().Context(), followerID, followeeID)
	if err != nil {
		return mapError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	code := http.StatusOK
	if status == "pending" {
		code = http.StatusAccepted
	}
	return c.JSON(code, followStateJSON{FollowStatus: status, IsFollowing: status == "following"})
}

func (h *Handler) Unfollow(c echo.Context) error {
	followerID, err := h.requireUser(c)
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
	return h.list(c, true)
}

func (h *Handler) ListFollowers(c echo.Context) error {
	return h.list(c, false)
}

func (h *Handler) list(c echo.Context, following bool) error {
	subjectID, err := uuid.Parse(c.Param("id"))
	if err != nil || subjectID == uuid.Nil {
		return mapError(contract.FieldError{Field: "id"})
	}
	viewerID, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	limit, offset, err := parsePage(c)
	if err != nil {
		return mapError(err)
	}
	var items []ports.FollowUser
	if following {
		items, err = h.listFollowing.Handle(c.Request().Context(), subjectID, viewerID, limit, offset)
	} else {
		items, err = h.listFollowers.Handle(c.Request().Context(), subjectID, viewerID, limit, offset)
	}
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, listResponse{Items: followItems(items)})
}

func (h *Handler) requireUser(c echo.Context) (uuid.UUID, error) {
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

func (h *Handler) optionalUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		return uuid.Nil, nil
	}
	if h.users == nil {
		return uuid.Nil, nil
	}
	userID, err := h.users.UserID(c.Request().Context(), id.TelegramID)
	if err != nil {
		return uuid.Nil, httperr.Unauthorized("unauthorized")
	}
	return userID, nil
}

func parsePage(c echo.Context) (limit, offset int, err error) {
	limit = 20
	if raw := c.QueryParam("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 {
			return 0, 0, contract.FieldError{Field: "limit"}
		}
		limit = n
	}
	if raw := c.QueryParam("offset"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 0 {
			return 0, 0, contract.FieldError{Field: "offset"}
		}
		offset = n
	}
	return limit, offset, nil
}

type followJSON struct {
	UserID      string    `json:"userId"`
	Username    string    `json:"username"`
	PhotoURL    string    `json:"photoUrl,omitempty"`
	IsFollowing bool      `json:"isFollowing"`
	CreatedAt   time.Time `json:"createdAt"`
}

type listResponse struct {
	Items []followJSON `json:"items"`
}

func followItems(items []ports.FollowUser) []followJSON {
	out := make([]followJSON, 0, len(items))
	for _, item := range items {
		out = append(out, followJSON{
			UserID:      item.UserID.String(),
			Username:    item.Username,
			PhotoURL:    item.PhotoURL,
			IsFollowing: item.IsFollowing,
			CreatedAt:   item.CreatedAt,
		})
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
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "not found")
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
