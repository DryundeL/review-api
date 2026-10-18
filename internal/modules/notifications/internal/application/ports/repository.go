package ports

import (
	"context"

	"github.com/google/uuid"
)

type PrefItem struct {
	Name    string
	Title   string
	Text    string
	Enabled bool
}

type PrefPatch struct {
	Name    string
	Enabled bool
}

type PrefRepository interface {
	EnsureDefaults(ctx context.Context, userID uuid.UUID) error
	List(ctx context.Context, userID uuid.UUID) ([]PrefItem, error)
	Patch(ctx context.Context, userID uuid.UUID, like, follow *bool) error
}
