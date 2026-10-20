package reviewshttp

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

type reviewBody struct {
	MediaObjectID string            `json:"mediaObjectId"`
	Title         string            `json:"title"`
	Rating        int16             `json:"rating"`
	Status        string            `json:"status"`
	Blocks        []reviewBlockBody `json:"blocks"`
}

type reviewBlockBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type voteBody struct {
	IsLike bool `json:"isLike"`
}

type reportBody struct {
	ReasonCode string `json:"reasonCode"`
}

type reportJSON struct {
	ID         string `json:"id"`
	ReviewID   string `json:"reviewId"`
	ReasonCode string `json:"reasonCode"`
	CreatedAt  string `json:"createdAt"`
}

type reasonJSON struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type reasonsListResponse struct {
	Items []reasonJSON `json:"items"`
}

type reviewJSON struct {
	ID            string           `json:"id"`
	AuthorID      string           `json:"authorId"`
	Author        *authorJSON      `json:"author,omitempty"`
	MediaObject   *mediaObjectJSON `json:"mediaObject,omitempty"`
	Title         string           `json:"title"`
	Rating        int16            `json:"rating"`
	Status        string           `json:"status,omitempty"`
	Blocks        []blockJSON      `json:"blocks,omitempty"`
	HasSpoilers   bool             `json:"hasSpoilers"`
	LikesCount    int64            `json:"likesCount"`
	DislikesCount int64            `json:"dislikesCount"`
	ViewsCount    int64            `json:"viewsCount"`
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

type blockJSON struct {
	ID       string `json:"id"`
	Position int16  `json:"position"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type reviewsListResponse struct {
	Items []reviewJSON `json:"items"`
}

func (h *Handler) CreateReview(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	var body reviewBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	draft, err := parseReviewDraft(body, true)
	if err != nil {
		return mapReviewError(err)
	}
	item, err := h.createReview.Handle(c.Request().Context(), userID, draft)
	if err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, reviewResponse(item))
}

func (h *Handler) UpdateReview(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	var body reviewBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	draft, err := parseReviewDraft(body, false)
	if err != nil {
		return mapReviewError(err)
	}
	item, err := h.updateReview.Handle(c.Request().Context(), id, userID, draft)
	if err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, reviewResponse(item))
}

func (h *Handler) GetReview(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	viewerID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	item, err := h.getReview.Handle(c.Request().Context(), id, viewerID)
	if err != nil {
		return mapReviewError(err)
	}
	resp := reviewResponse(item)
	resp.Status = ""
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListReviews(c echo.Context) error {
	mediaID, err := uuid.Parse(c.QueryParam("mediaObjectId"))
	if err != nil {
		return mapReviewError(contract.FieldError{Field: "mediaObjectId"})
	}
	items, err := h.listReviews.Handle(c.Request().Context(), mediaID, parseLimit(c))
	if err != nil {
		return mapReviewError(err)
	}
	return c.JSON(http.StatusOK, reviewsListResponse{Items: reviewsResponse(items)})
}

func (h *Handler) ListMyReviews(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	items, err := h.listReviews.ByAuthor(c.Request().Context(), userID, false, parseLimit(c))
	if err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, reviewsListResponse{Items: reviewsResponse(items)})
}

func (h *Handler) ListUserReviews(c echo.Context) error {
	authorID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return mapReviewError(contract.FieldError{Field: "authorId"})
	}
	items, err := h.listReviews.ByAuthor(c.Request().Context(), authorID, true, parseLimit(c))
	if err != nil {
		return mapReviewError(err)
	}
	return c.JSON(http.StatusOK, reviewsListResponse{Items: reviewsResponse(items)})
}

func (h *Handler) DeleteReview(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	if err := h.deleteReview.Handle(c.Request().Context(), id, userID); err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) VoteReview(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	var body voteBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	if err := h.voteReview.Handle(c.Request().Context(), id, userID, body.IsLike); err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ViewReview(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	if err := h.viewReview.Handle(c.Request().Context(), id, userID); err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ListReportReasons(c echo.Context) error {
	items, err := h.listReasons.Handle(c.Request().Context())
	if err != nil {
		return mapReviewError(err)
	}
	out := make([]reasonJSON, 0, len(items))
	for _, item := range items {
		out = append(out, reasonJSON{Code: item.Code, Label: item.Label})
	}
	return c.JSON(http.StatusOK, reasonsListResponse{Items: out})
}

func (h *Handler) CreateReport(c echo.Context) error {
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	var body reportBody
	if err := c.Bind(&body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	item, err := h.createReport.Handle(c.Request().Context(), id, userID, body.ReasonCode)
	if err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, reportJSON{
		ID:         item.ID.String(),
		ReviewID:   item.ReviewID.String(),
		ReasonCode: item.ReasonCode,
		CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
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

func parseReviewDraft(body reviewBody, requireMedia bool) (domain.Draft, error) {
	var mediaID uuid.UUID
	if requireMedia || body.MediaObjectID != "" {
		parsed, err := uuid.Parse(body.MediaObjectID)
		if err != nil {
			return domain.Draft{}, contract.FieldError{Field: "mediaObjectId"}
		}
		mediaID = parsed
	}
	blocks := make([]domain.BlockDraft, 0, len(body.Blocks))
	for _, b := range body.Blocks {
		blocks = append(blocks, domain.BlockDraft{Title: b.Title, Body: b.Body})
	}
	return domain.Draft{
		MediaObjectID: mediaID,
		Title:         body.Title,
		Rating:        body.Rating,
		Status:        body.Status,
		Blocks:        blocks,
	}, nil
}

func parseLimit(c echo.Context) int {
	raw := c.QueryParam("limit")
	if raw == "" {
		return 20
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 20
	}
	return n
}

func reviewsResponse(items []ports.Review) []reviewJSON {
	out := make([]reviewJSON, 0, len(items))
	for _, item := range items {
		out = append(out, reviewListItem(item))
	}
	return out
}

func reviewListItem(item ports.Review) reviewJSON {
	return reviewJSON{
		ID:            item.ID.String(),
		AuthorID:      item.AuthorID.String(),
		Author:        authorResponse(item.Author),
		MediaObject:   mediaObjectResponse(item.MediaObject),
		Title:         item.Title,
		Rating:        item.Rating,
		Status:        item.Status,
		HasSpoilers:   item.HasSpoilers,
		LikesCount:    item.LikesCount,
		DislikesCount: item.DislikesCount,
		ViewsCount:    item.ViewsCount,
	}
}

func reviewResponse(item ports.Review) reviewJSON {
	blocks := make([]blockJSON, 0, len(item.Blocks))
	for _, b := range item.Blocks {
		blocks = append(blocks, blockJSON{
			ID: b.ID.String(), Position: b.Position, Title: b.Title, Body: b.Body,
		})
	}
	return reviewJSON{
		ID:            item.ID.String(),
		AuthorID:      item.AuthorID.String(),
		Author:        authorResponse(item.Author),
		MediaObject:   mediaObjectResponse(item.MediaObject),
		Title:         item.Title,
		Rating:        item.Rating,
		Status:        item.Status,
		Blocks:        blocks,
		HasSpoilers:   item.HasSpoilers,
		LikesCount:    item.LikesCount,
		DislikesCount: item.DislikesCount,
		ViewsCount:    item.ViewsCount,
	}
}

func authorResponse(a ports.AuthorRef) *authorJSON {
	if a.ID == uuid.Nil {
		return nil
	}
	return &authorJSON{
		ID:          a.ID.String(),
		Username:    a.Username,
		PhotoURL:    a.PhotoURL,
		IsFollowing: a.IsFollowing,
		AvatarFrame: a.AvatarFrame,
		Backdrop:    a.Backdrop,
	}
}

func mediaObjectResponse(mo ports.MediaObjectRef) *mediaObjectJSON {
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

func mapReviewError(err error) error {
	var field contract.FieldError
	switch {
	case errors.As(err, &field):
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid review",
			Details: map[string]string{"field": field.Field},
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	case errors.Is(err, contract.ErrAlreadyReported):
		return httperr.New(http.StatusConflict, "ALREADY_REPORTED", "review already reported")
	case errors.Is(err, contract.ErrConflict):
		return httperr.New(http.StatusConflict, "CONFLICT", "review already published for this title")
	case errors.Is(err, contract.ErrForbidden):
		return httperr.New(http.StatusForbidden, "FORBIDDEN", "forbidden")
	case errors.Is(err, contract.ErrInvalid):
		return httperr.New(http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid review")
	default:
		return err
	}
}
