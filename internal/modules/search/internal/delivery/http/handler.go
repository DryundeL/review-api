package searchhttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/search/contract"
	"review-api/internal/modules/search/internal/application/ports"
	"review-api/internal/modules/search/internal/application/query"
)

type UserResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Handler struct {
	trends *query.TrendsHandler
	search *query.SearchHandler
	users  UserResolver
}

func NewHandler(trends *query.TrendsHandler, search *query.SearchHandler, users UserResolver) *Handler {
	return &Handler{trends: trends, search: search, users: users}
}

type mediaTrendJSON struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	PosterURL    string `json:"posterUrl,omitempty"`
	ReviewsCount int64  `json:"reviewsCount"`
}

type authorTrendJSON struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	FirstName      string `json:"firstName"`
	FollowersCount int64  `json:"followersCount"`
	IsFollowing    bool   `json:"isFollowing"`
}

type typeTrendJSON struct {
	Code         string `json:"code"`
	Label        string `json:"label"`
	ReviewsCount int64  `json:"reviewsCount"`
}

type userHitJSON struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	FirstName      string `json:"firstName"`
	FollowersCount int64  `json:"followersCount"`
	IsFollowing    bool   `json:"isFollowing"`
}

type mediaRefJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	PosterURL string `json:"posterUrl,omitempty"`
}

type reviewHitJSON struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	AuthorID    string       `json:"authorId"`
	Username    string       `json:"username"`
	MediaObject mediaRefJSON `json:"mediaObject"`
	Rating      int16        `json:"rating"`
	LikesCount  int64        `json:"likesCount"`
	ViewsCount  int64        `json:"viewsCount"`
}

type mediaHitJSON struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	PosterURL string `json:"posterUrl,omitempty"`
}

func (h *Handler) Trends(c echo.Context) error {
	viewerID, err := h.resolveViewer(c)
	if err != nil {
		return err
	}
	result, err := h.trends.Handle(c.Request().Context(), query.TrendsInput{
		Tab:      c.QueryParam("tab"),
		Limit:    parseLimit(c),
		ViewerID: viewerID,
	})
	if err != nil {
		return mapError(err)
	}

	switch result.Tab {
	case ports.TabAuthors:
		items := make([]authorTrendJSON, 0, len(result.Authors))
		for _, a := range result.Authors {
			items = append(items, authorTrendJSON{
				ID: a.ID.String(), Username: a.Username, FirstName: a.FirstName,
				FollowersCount: a.FollowersCount, IsFollowing: a.IsFollowing,
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"tab": result.Tab, "items": items})
	case ports.TabTypes:
		items := make([]typeTrendJSON, 0, len(result.Types))
		for _, t := range result.Types {
			items = append(items, typeTrendJSON{Code: t.Code, Label: t.Label, ReviewsCount: t.ReviewsCount})
		}
		return c.JSON(http.StatusOK, map[string]any{"tab": result.Tab, "items": items})
	default:
		items := make([]mediaTrendJSON, 0, len(result.Media))
		for _, m := range result.Media {
			items = append(items, mediaTrendJSON{
				ID: m.ID.String(), Title: m.Title, Type: m.Type,
				PosterURL: m.PosterURL, ReviewsCount: m.ReviewsCount,
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"tab": result.Tab, "items": items})
	}
}

func (h *Handler) Search(c echo.Context) error {
	viewerID, err := h.resolveViewer(c)
	if err != nil {
		return err
	}
	result, err := h.search.Handle(c.Request().Context(), query.SearchInput{
		Q:        c.QueryParam("q"),
		Limit:    parseLimit(c),
		ViewerID: viewerID,
	})
	if err != nil {
		return mapError(err)
	}

	users := make([]userHitJSON, 0, len(result.Users))
	for _, u := range result.Users {
		users = append(users, userHitJSON{
			ID: u.ID.String(), Username: u.Username, FirstName: u.FirstName,
			FollowersCount: u.FollowersCount, IsFollowing: u.IsFollowing,
		})
	}
	reviews := make([]reviewHitJSON, 0, len(result.Reviews))
	for _, r := range result.Reviews {
		reviews = append(reviews, reviewHitJSON{
			ID: r.ID.String(), Title: r.Title, AuthorID: r.AuthorID.String(), Username: r.Username,
			MediaObject: mediaRefJSON{
				ID: r.MediaObject.ID.String(), Title: r.MediaObject.Title,
				Type: r.MediaObject.Type, PosterURL: r.MediaObject.PosterURL,
			},
			Rating: r.Rating, LikesCount: r.LikesCount, ViewsCount: r.ViewsCount,
		})
	}
	media := make([]mediaHitJSON, 0, len(result.Media))
	for _, m := range result.Media {
		media = append(media, mediaHitJSON{
			ID: m.ID.String(), Title: m.Title, Type: m.Type, PosterURL: m.PosterURL,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"users": users, "reviews": reviews, "mediaObjects": media,
	})
}

func (h *Handler) resolveViewer(c echo.Context) (uuid.UUID, error) {
	if c.Request().Header.Get(echo.HeaderAuthorization) == "" {
		return uuid.Nil, nil
	}
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
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func mapError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid search query",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid search query")
	default:
		return err
	}
}
