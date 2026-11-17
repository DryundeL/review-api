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
	gamificationcontract "review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

// maxWriteBodyBytes caps editor requests: 20 000 text characters say nothing about markup size.
const maxWriteBodyBytes = "256K"

// Only editable fields are bound; author, status and counters never come from the client.
type createReviewBody struct {
	MediaObjectID string `json:"mediaObjectId"`
	Title         string `json:"title"`
	Rating        *int16 `json:"rating"`
	Body          string `json:"body"`
}

type saveDraftBody struct {
	Title    string `json:"title"`
	Rating   *int16 `json:"rating"`
	Body     string `json:"body"`
	Revision int64  `json:"revision"`
}

type publishBody struct {
	Revision int64 `json:"revision"`
}

type editorJSON struct {
	ID                    string           `json:"id"`
	MediaObject           *mediaObjectJSON `json:"mediaObject,omitempty"`
	Status                string           `json:"status"`
	Title                 string           `json:"title"`
	Rating                *int16           `json:"rating"`
	Body                  string           `json:"body"`
	Revision              int64            `json:"revision"`
	HasUnpublishedChanges bool             `json:"hasUnpublishedChanges"`
	PublishedAt           *string          `json:"publishedAt,omitempty"`
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
	ID          string           `json:"id"`
	AuthorID    string           `json:"authorId"`
	Author      *authorJSON      `json:"author,omitempty"`
	MediaObject *mediaObjectJSON `json:"mediaObject,omitempty"`
	Title       string           `json:"title"`
	Rating      *int16           `json:"rating"`
	Status      string           `json:"status,omitempty"`
	Body        string           `json:"body,omitempty"`
	HasSpoilers bool             `json:"hasSpoilers"`
	// Only in the author's own list.
	HasUnpublishedChanges bool  `json:"hasUnpublishedChanges,omitempty"`
	LikesCount            int64 `json:"likesCount"`
	DislikesCount         int64 `json:"dislikesCount"`
	ViewsCount            int64 `json:"viewsCount"`
	ViewerIsLike          *bool `json:"viewerIsLike,omitempty"`
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

type reviewsListResponse struct {
	Items []reviewJSON `json:"items"`
}

func (h *Handler) GetOwnReview(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	mediaID, err := uuid.Parse(c.QueryParam("mediaObjectId"))
	if err != nil {
		return mapReviewError(contract.FieldError{Field: "mediaObjectId"})
	}
	item, err := h.editorReview.ByMedia(c.Request().Context(), userID, mediaID)
	if err != nil {
		return mapReviewError(err)
	}
	return c.JSON(http.StatusOK, editorResponse(item))
}

func (h *Handler) CreateReview(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	var body createReviewBody
	if err := c.Bind(&body); err != nil {
		return bindError(err)
	}
	mediaID, err := uuid.Parse(body.MediaObjectID)
	if err != nil {
		return mapReviewError(contract.FieldError{Field: "mediaObjectId"})
	}
	draft := domain.Draft{Title: body.Title, Rating: body.Rating, Body: body.Body}
	item, created, err := h.createReview.Handle(c.Request().Context(), userID, mediaID, draft)
	if err != nil {
		return mapReviewError(err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.JSON(status, editorResponse(item))
}

func (h *Handler) SaveReviewDraft(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	var body saveDraftBody
	if err := c.Bind(&body); err != nil {
		return bindError(err)
	}
	draft := domain.Draft{Title: body.Title, Rating: body.Rating, Body: body.Body}
	item, err := h.saveDraft.Handle(c.Request().Context(), id, userID, body.Revision, draft)
	if err != nil {
		return mapReviewError(err)
	}
	return c.JSON(http.StatusOK, editorResponse(item))
}

func (h *Handler) PublishReview(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	userID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	var body publishBody
	if err := c.Bind(&body); err != nil {
		return bindError(err)
	}
	item, err := h.publishReview.Handle(c.Request().Context(), id, userID, body.Revision)
	if err != nil {
		return mapReviewError(err)
	}
	return c.JSON(http.StatusOK, editorResponse(item))
}

// bindError keeps the 413 from the body limit instead of masking it as 400.
func bindError(err error) error {
	var he *echo.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusRequestEntityTooLarge {
		return err
	}
	return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
}

func (h *Handler) GetReview(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	}
	viewerID, err := h.optionalUser(c)
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
	viewerID, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	items, err := h.listReviews.Handle(c.Request().Context(), mediaID, viewerID, parseLimit(c))
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
	items, err := h.listReviews.ByAuthor(c.Request().Context(), userID, userID, false, parseLimit(c))
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
	viewerID, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	items, err := h.listReviews.ByAuthor(c.Request().Context(), authorID, viewerID, true, parseLimit(c))
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

// optionalUser returns viewer id when Bearer is present; uuid.Nil without auth.
func (h *Handler) optionalUser(c echo.Context) (uuid.UUID, error) {
	id, err := authcontract.IdentityFromContext(c.Request().Context())
	if err != nil {
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
		ViewerIsLike:  item.ViewerIsLike,

		HasUnpublishedChanges: item.HasUnpublishedChanges,
	}
}

func reviewResponse(item ports.Review) reviewJSON {
	return reviewJSON{
		ID:            item.ID.String(),
		AuthorID:      item.AuthorID.String(),
		Author:        authorResponse(item.Author),
		MediaObject:   mediaObjectResponse(item.MediaObject),
		Title:         item.Title,
		Rating:        item.Rating,
		Status:        item.Status,
		Body:          item.Body,
		HasSpoilers:   item.HasSpoilers,
		LikesCount:    item.LikesCount,
		DislikesCount: item.DislikesCount,
		ViewsCount:    item.ViewsCount,
		ViewerIsLike:  item.ViewerIsLike,
	}
}

func editorResponse(item ports.EditorReview) editorJSON {
	out := editorJSON{
		ID:                    item.ID.String(),
		MediaObject:           mediaObjectResponse(item.MediaObject),
		Status:                item.Status,
		Title:                 item.Title,
		Rating:                item.Rating,
		Body:                  item.Body,
		Revision:              item.Revision,
		HasUnpublishedChanges: item.HasUnpublishedChanges,
	}
	if item.PublishedAt != nil {
		at := item.PublishedAt.UTC().Format(time.RFC3339Nano)
		out.PublishedAt = &at
	}
	return out
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
		AvatarFrame: gamificationcontract.CosmeticURL(a.AvatarFrame),
		Backdrop:    gamificationcontract.CosmeticURL(a.Backdrop),
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
		details := map[string]string{"field": field.Field}
		if field.Reason != "" {
			details["reason"] = field.Reason
		}
		return &httperr.Error{
			Status:  http.StatusUnprocessableEntity,
			Code:    "VALIDATION_ERROR",
			Message: "invalid review",
			Details: details,
		}
	case errors.Is(err, contract.ErrNotFound):
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "review not found")
	case errors.Is(err, contract.ErrRevisionConflict):
		return httperr.New(http.StatusConflict, "REVISION_CONFLICT", "review was changed elsewhere")
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

func (h *Handler) TopReviews(c echo.Context) error {
	mediaID, err := uuid.Parse(c.Param("id"))
	if err != nil || mediaID == uuid.Nil {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "media object not found")
	}
	viewerID, err := h.optionalUser(c)
	if err != nil {
		return err
	}
	items, err := h.listReviews.TopByMedia(c.Request().Context(), mediaID, viewerID)
	if errors.Is(err, contract.ErrNotFound) {
		return httperr.New(http.StatusNotFound, "NOT_FOUND", "media object not found")
	}
	if err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, reviewsListResponse{Items: reviewsResponse(items)})
}
