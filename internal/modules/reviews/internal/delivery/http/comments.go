package reviewshttp

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"review-api/internal/http/httperr"
	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

type commentJSON struct {
	ID         uuid.UUID   `json:"id"`
	ReviewID   uuid.UUID   `json:"reviewId"`
	ParentID   *uuid.UUID  `json:"parentId"`
	Author     *authorJSON `json:"author"`
	Body       string      `json:"body"`
	CreatedAt  string      `json:"createdAt"`
	HasReplies bool        `json:"hasReplies"`
}

func commentResponse(item ports.Comment) commentJSON {
	return commentJSON{ID: item.ID, ReviewID: item.ReviewID, ParentID: item.ParentID, Author: authorResponse(item.Author), Body: item.Body, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), HasReplies: item.HasReplies}
}

func commentReviewID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, mapReviewError(contract.FieldError{Field: "reviewId"})
	}
	return id, nil
}

func (h *Handler) CreateComment(c echo.Context) error {
	authorID, err := h.resolveUser(c)
	if err != nil {
		return err
	}
	reviewID, err := commentReviewID(c)
	if err != nil {
		return err
	}
	// Read a bounded raw body before JSON decoding, which otherwise replaces invalid UTF-8.
	data, err := io.ReadAll(http.MaxBytesReader(c.Response(), c.Request().Body, 32*1024))
	if err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	if !utf8.Valid(data) {
		return mapReviewError(contract.FieldError{Field: "body"})
	}
	var body struct {
		Body     string  `json:"body"`
		ParentID *string `json:"parentId"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return httperr.New(http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
	}
	var parentID *uuid.UUID
	if body.ParentID != nil {
		id, err := uuid.Parse(*body.ParentID)
		if err != nil || id == uuid.Nil {
			return mapReviewError(contract.FieldError{Field: "parentId"})
		}
		parentID = &id
	}
	item, err := h.createComment.Handle(c.Request().Context(), reviewID, authorID, parentID, body.Body)
	if err != nil {
		return mapReviewError(err)
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusCreated, commentResponse(item))
}

func (h *Handler) ListComments(c echo.Context) error {
	reviewID, err := commentReviewID(c)
	if err != nil {
		return err
	}
	limit := 20
	if raw, ok := c.QueryParams()["limit"]; ok {
		if len(raw) != 1 {
			return mapReviewError(contract.FieldError{Field: "limit"})
		}
		limit, err = strconv.Atoi(raw[0])
		if err != nil {
			return mapReviewError(contract.FieldError{Field: "limit"})
		}
	}
	var parentID *uuid.UUID
	if raw, ok := c.QueryParams()["parentId"]; ok {
		if len(raw) != 1 {
			return mapReviewError(contract.FieldError{Field: "parentId"})
		}
		id, err := uuid.Parse(raw[0])
		if err != nil || id == uuid.Nil {
			return mapReviewError(contract.FieldError{Field: "parentId"})
		}
		parentID = &id
	}
	page, err := h.listComments.Handle(c.Request().Context(), reviewID, parentID, limit, c.QueryParam("cursor"))
	if err != nil {
		return mapReviewError(err)
	}
	items := make([]commentJSON, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, commentResponse(item))
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, struct {
		Items      []commentJSON `json:"items"`
		NextCursor *string       `json:"nextCursor"`
	}{Items: items, NextCursor: page.NextCursor})
}
