package ports

import (
	"context"

	"github.com/google/uuid"
)

type Reader interface {
	List(ctx context.Context, entity string, after uuid.UUID, limit int) ([]uuid.UUID, error)
}
