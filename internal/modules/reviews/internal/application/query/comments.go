package query

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

type CommentPage struct {
	Items      []ports.Comment
	NextCursor *string
}

type ListCommentsHandler struct{ reader ports.CommentReader }

func NewListCommentsHandler(reader ports.CommentReader) *ListCommentsHandler {
	return &ListCommentsHandler{reader: reader}
}

func (h *ListCommentsHandler) Handle(ctx context.Context, reviewID uuid.UUID, parentID *uuid.UUID, limit int, cursor string) (CommentPage, error) {
	if reviewID == uuid.Nil {
		return CommentPage{}, contract.FieldError{Field: "reviewId"}
	}
	if parentID != nil && *parentID == uuid.Nil {
		return CommentPage{}, contract.FieldError{Field: "parentId"}
	}
	if limit < 1 || limit > 50 {
		return CommentPage{}, contract.FieldError{Field: "limit"}
	}
	after, err := decodeCommentCursor(cursor, reviewID, parentID)
	if err != nil {
		return CommentPage{}, err
	}
	items, err := h.reader.ListComments(ctx, reviewID, parentID, after, limit+1)
	if err != nil {
		return CommentPage{}, err
	}
	page := CommentPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		next := encodeCommentCursor(reviewID, parentID, ports.CommentPosition{ID: last.ID, CreatedAt: last.CreatedAt})
		page.NextCursor = &next
	}
	return page, nil
}

type commentCursor struct {
	ReviewID uuid.UUID             `json:"r"`
	ParentID *uuid.UUID            `json:"p"`
	Position ports.CommentPosition `json:"s"`
}

func encodeCommentCursor(reviewID uuid.UUID, parentID *uuid.UUID, pos ports.CommentPosition) string {
	data, _ := json.Marshal(commentCursor{ReviewID: reviewID, ParentID: parentID, Position: pos})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCommentCursor(raw string, reviewID uuid.UUID, parentID *uuid.UUID) (*ports.CommentPosition, error) {
	if raw == "" {
		return nil, nil
	}
	invalid := contract.FieldError{Field: "cursor"}
	if len(raw) > 1024 {
		return nil, invalid
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalid
	}
	var cursor commentCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, invalid
	}
	if cursor.ReviewID != reviewID || (cursor.ParentID == nil) != (parentID == nil) ||
		(parentID != nil && *cursor.ParentID != *parentID) || cursor.Position.ID == uuid.Nil || cursor.Position.CreatedAt.IsZero() {
		return nil, invalid
	}
	return &cursor.Position, nil
}
