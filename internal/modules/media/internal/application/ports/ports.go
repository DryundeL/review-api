package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ImageRecord is a stored upload as persisted in media_images.
type ImageRecord struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	StorageKey  string
	ContentType string
	SizeBytes   int
	Width       int
	Height      int
	CreatedAt   time.Time
}

type ImageWriter interface {
	Insert(ctx context.Context, item ImageRecord) error
	// LockUnattached locks up to limit unattached rows created before cutoff (SKIP LOCKED). Requires a TX.
	LockUnattached(ctx context.Context, cutoff time.Time, limit int) ([]ImageRecord, error)
	DeleteByIDs(ctx context.Context, ids []uuid.UUID) error
}

// FileStore persists image bytes under a server-generated key.
type FileStore interface {
	Put(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
}

// Quota counts uploads per user in a fixed window.
type Quota interface {
	Allow(ctx context.Context, userID uuid.UUID) (bool, error)
}
