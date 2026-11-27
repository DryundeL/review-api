package query

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
)

type CommentPage struct {
	Items      []ports.Comment
	NextCursor string // empty on the last page
}

type CommentsHandler struct {
	repo ports.CommentReader
}

func NewCommentsHandler(repo ports.CommentReader) *CommentsHandler {
	return &CommentsHandler{repo: repo}
}

// List returns one page of a thread: top-level comments, or the replies to parentID, oldest first.
// limit 0 means DefaultPageSize.
func (h *CommentsHandler) List(ctx context.Context, resultID uuid.UUID, parentID *uuid.UUID, cursor string, limit int) (CommentPage, error) {
	if h == nil || h.repo == nil {
		return CommentPage{}, contract.ErrInvalid
	}
	if resultID == uuid.Nil {
		return CommentPage{}, contract.ErrNotFound
	}
	if parentID != nil && *parentID == uuid.Nil {
		return CommentPage{}, contract.Field("parentId", contract.ReasonInvalid)
	}
	if limit == 0 {
		limit = DefaultPageSize
	}
	if limit < 1 || limit > MaxPageSize {
		return CommentPage{}, contract.Field("limit", contract.ReasonInvalid)
	}
	after, err := decodeCommentCursor(cursor, resultID, parentID)
	if err != nil {
		return CommentPage{}, err
	}
	items, err := h.repo.ListComments(ctx, resultID, parentID, after, limit+1)
	if err != nil {
		return CommentPage{}, err
	}
	page := CommentPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		page.NextCursor = encodeCommentCursor(commentCursor{
			ResultID: resultID, ParentID: parentID,
			Position: ports.CommentPosition{CreatedAt: last.CreatedAt.UTC(), ID: last.ID},
		})
	}
	return page, nil
}

// commentCursor is bound to the thread, so it cannot be replayed on another result or parent.
type commentCursor struct {
	ResultID uuid.UUID             `json:"r"`
	ParentID *uuid.UUID            `json:"p,omitempty"`
	Position ports.CommentPosition `json:"s"`
}

func encodeCommentCursor(c commentCursor) string {
	data, _ := json.Marshal(c) // fixed struct of UUIDs and a time: cannot fail
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCommentCursor(raw string, resultID uuid.UUID, parentID *uuid.UUID) (*ports.CommentPosition, error) {
	if raw == "" {
		return nil, nil
	}
	invalid := contract.Field("cursor", contract.ReasonInvalid)
	if len(raw) > maxCursorLen {
		return nil, invalid
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, invalid
	}
	var c commentCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, invalid
	}
	sameParent := (c.ParentID == nil) == (parentID == nil) && (parentID == nil || *c.ParentID == *parentID)
	if c.ResultID != resultID || !sameParent || c.Position.ID == uuid.Nil || c.Position.CreatedAt.IsZero() {
		return nil, invalid
	}
	return &c.Position, nil
}
