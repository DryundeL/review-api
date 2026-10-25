package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/catalog/internal/domain"
)

type MediaObject struct {
	ID             uuid.UUID
	Title          string
	OriginalTitle  string
	Type           string
	Info           domain.Info
	PosterURL      string
	ExternalSource string
	ExternalID     string
	CreatedBy      *uuid.UUID
}

type Writer interface {
	Save(ctx context.Context, item domain.MediaObject) error
}

type Reader interface {
	Get(ctx context.Context, id uuid.UUID) (MediaObject, error)
	Search(ctx context.Context, query, typeCode string, limit int) ([]MediaObject, error)
}

type TypeChecker interface {
	RequireActive(ctx context.Context, codes []string) error
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
