package feedhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/feed/contract"
	"review-api/internal/modules/feed/internal/application/ports"
	"review-api/internal/modules/feed/internal/application/query"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	list  *query.ListFeedHandler
	users UserResolver
}

func NewHandler(list *query.ListFeedHandler, users UserResolver) *Handler {
	return &Handler{list: list, users: users}
}

type feedItemJSON struct {
	ID            string           `json:"id"`
	AuthorID      string           `json:"authorId"`
	Author        authorJSON       `json:"author"`
	MediaObject   *mediaObjectJSON `json:"mediaObject,omitempty"`
	Title         string           `json:"title"`
	Rating        int16            `json:"rating"`
	HasSpoilers   bool             `json:"hasSpoilers"`
	LikesCount    int64            `json:"likesCount"`
	DislikesCount int64            `json:"dislikesCount"`
	ViewsCount    int64            `json:"viewsCount"`
	ViewerIsLike  *bool            `json:"viewerIsLike,omitempty"`
}

type authorJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	PhotoURL    string `json:"photoUrl,omitempty"`
	IsFollowing bool   `json:"isFollowing"`
	AvatarFrame string `json:"avatarFrame,omitempty"`
	Backdrop    string `json:"backdrop,omitempty"`
}

type mediaObjectJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Type      string `json:"type,omitempty"`
	PosterURL string `json:"posterUrl,omitempty"`
}

type feedResponse struct {
	Items      []feedItemJSON `json:"items"`
	NextCursor *string        `json:"nextCursor"`
}

func (h *Handler) ListFeed(c echo.Context) error {
	mode := c.QueryParam("mode")
	viewerID, err := h.resolveViewer(c, mode == ports.ModeFollowing)
	if err != nil {
		return err
	}

	page, err := h.list.Handle(c.Request().Context(), query.ListInput{
		Sort:     c.QueryParam("sort"),
		Mode:     mode,
		Type:     c.QueryParam("type"),
		Cursor:   c.QueryParam("cursor"),
		Limit:    parseLimit(c),
		ViewerID: viewerID,
	})
	if err != nil {
		return mapError(err)
	}

	items := make([]feedItemJSON, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, itemJSON(item))
	}
	var next *string
	if page.NextCursor != "" {
		next = &page.NextCursor
	}
	return c.JSON(http.StatusOK, feedResponse{Items: items, NextCursor: next})
}

// resolveViewer returns viewer id. required=true → 401 without auth.
func (h *Handler) resolveViewer(c echo.Context, required bool) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
		if required {
			return uuid.Nil, httperr.Unauthorized("unauthorized")
		}
		return uuid.Nil, nil
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
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func itemJSON(item ports.Item) feedItemJSON {
	return feedItemJSON{
		ID:       item.ID.String(),
		AuthorID: item.AuthorID.String(),
		Author: authorJSON{
			ID:          item.Author.ID.String(),
			Username:    item.Author.Username,
			PhotoURL:    item.Author.PhotoURL,
			IsFollowing: item.Author.IsFollowing,
			AvatarFrame: item.Author.AvatarFrame,
			Backdrop:    item.Author.Backdrop,
		},
		MediaObject:   mediaJSON(item.MediaObject),
		Title:         item.Title,
		Rating:        item.Rating,
		HasSpoilers:   item.HasSpoilers,
		LikesCount:    item.LikesCount,
		DislikesCount: item.DislikesCount,
		ViewsCount:    item.ViewsCount,
		ViewerIsLike:  item.ViewerIsLike,
	}
}

func mediaJSON(mo ports.MediaObjectRef) *mediaObjectJSON {
	if mo.ID == uuid.Nil {
		return nil
	}
	return &mediaObjectJSON{
		ID:        mo.ID.String(),
		Title:     mo.Title,
		Type:      mo.Type,
		PosterURL: mo.PosterURL,
	}
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid feed query",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrUnauthorized):
		return httperr.Unauthorized("unauthorized")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid feed query")
	default:
		return err
	}
}
