package domain

import (
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
)

// MaxCommentLen matches the limit of review comments.
const MaxCommentLen = 2000

// Comment is a comment on a published result. ParentID is set for a reply.
type Comment struct {
	ID        uuid.UUID
	ResultID  uuid.UUID
	AuthorID  uuid.UUID
	ParentID  *uuid.UUID
	Body      string
	CreatedAt time.Time
}

// NewComment validates the body: trimmed, 1..MaxCommentLen code points, no control characters
// other than newlines and tabs.
func NewComment(id, resultID, authorID uuid.UUID, parentID *uuid.UUID, body string, now time.Time) (Comment, error) {
	if id == uuid.Nil || resultID == uuid.Nil || authorID == uuid.Nil {
		return Comment{}, contract.ErrInvalid
	}
	if parentID != nil && *parentID == uuid.Nil {
		return Comment{}, contract.Field("parentId", contract.ReasonInvalid)
	}
	clean, err := cleanField(body, MaxCommentLen, true, "body", -1)
	if err != nil {
		return Comment{}, err
	}
	if clean == "" {
		return Comment{}, contract.Field("body", contract.ReasonRequired)
	}
	return Comment{
		ID: id, ResultID: resultID, AuthorID: authorID, ParentID: cloneID(parentID), Body: clean, CreatedAt: now.UTC(),
	}, nil
}
