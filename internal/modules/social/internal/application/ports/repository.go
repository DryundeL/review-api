package ports

import (
	"context"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/social/internal/domain"
)

type Follow struct {
	FollowerID uuid.UUID
	FolloweeID uuid.UUID
	CreatedAt  time.Time
}

type Writer interface {
	Create(ctx context.Context, item domain.Follow) error
	Delete(ctx context.Context, followerID, followeeID uuid.UUID) error
}

type Reader interface {
	ListFollowing(ctx context.Context, followerID uuid.UUID, limit int) ([]Follow, error)
	ListFollowers(ctx context.Context, followeeID uuid.UUID, limit int) ([]Follow, error)
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
