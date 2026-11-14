package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/internal/domain"
	"time"
)

type Comment struct {
	domain.Comment
	Author     AuthorRef
	HasReplies bool
}

type CommentPosition struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

type CommentWriter interface {
	CreateComment(context.Context, domain.Comment) error
}

type CommentReader interface {
	GetComment(context.Context, uuid.UUID, uuid.UUID) (Comment, error)
	ListComments(context.Context, uuid.UUID, *uuid.UUID, *CommentPosition, int) ([]Comment, error)
}
