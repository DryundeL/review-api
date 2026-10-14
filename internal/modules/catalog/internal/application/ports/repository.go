package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/catalog/internal/domain"
)

type Work struct {
	ID            uuid.UUID
	Title         string
	PrimaryType   string
	Types         []string
	ReleaseYear   *int
	CoverURL      string
	ReleaseStatus string
	Origin        string
	CreatedBy     uuid.UUID
}

type Writer interface {
	Save(ctx context.Context, work domain.Work) error
}

type Reader interface {
	Get(ctx context.Context, id uuid.UUID) (Work, error)
	Search(ctx context.Context, query, typeCode string, limit int) ([]Work, error)
}

type TypeChecker interface {
	RequireActive(ctx context.Context, codes []string) error
}

type AuthorResolver interface {
	UserID(ctx context.Context, telegramID int64) (uuid.UUID, error)
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
