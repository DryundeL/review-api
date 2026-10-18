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

// FollowUser is a list row with profile fields for followers/following screens.
type FollowUser struct {
	UserID      uuid.UUID
	Username    string
	PhotoURL    string
	IsFollowing bool
	CreatedAt   time.Time
}

type Writer interface {
	Create(ctx context.Context, item domain.Follow) error
	Delete(ctx context.Context, followerID, followeeID uuid.UUID) error
}

type Reader interface {
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	ListFollowing(ctx context.Context, subjectID, viewerID uuid.UUID, limit, offset int) ([]FollowUser, error)
	ListFollowers(ctx context.Context, subjectID, viewerID uuid.UUID, limit, offset int) ([]FollowUser, error)
}

type Publisher interface {
	Publish(ctx context.Context, eventName, aggregateID string, payload any) error
}
