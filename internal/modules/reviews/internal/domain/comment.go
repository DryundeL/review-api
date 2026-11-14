package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
)

type Comment struct {
	ID        uuid.UUID
	ReviewID  uuid.UUID
	AuthorID  uuid.UUID
	ParentID  *uuid.UUID
	Body      string
	CreatedAt time.Time
}

func NewComment(reviewID, authorID uuid.UUID, parentID *uuid.UUID, body string) (Comment, error) {
	if reviewID == uuid.Nil {
		return Comment{}, contract.FieldError{Field: "reviewId"}
	}
	if authorID == uuid.Nil {
		return Comment{}, contract.FieldError{Field: "authorId"}
	}
	if parentID != nil && *parentID == uuid.Nil {
		return Comment{}, contract.FieldError{Field: "parentId"}
	}
	body = strings.TrimSpace(body)
	if !utf8.ValidString(body) || strings.ContainsRune(body, 0) || utf8.RuneCountInString(body) < 1 || utf8.RuneCountInString(body) > 2000 {
		return Comment{}, contract.FieldError{Field: "body"}
	}
	if parentID != nil {
		p := *parentID
		parentID = &p
	}
	return Comment{ID: uuid.New(), ReviewID: reviewID, AuthorID: authorID, ParentID: parentID, Body: body, CreatedAt: time.Now().UTC()}, nil
}
