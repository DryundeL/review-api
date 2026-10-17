package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/users/internal/domain"
)

type UserRepository interface {
	Upsert(ctx context.Context, user domain.User) error
}

// ProfileCard is a user row plus follow stats for profile screens.
type ProfileCard struct {
	User           domain.User
	FollowersCount int64
	FollowingCount int64
	IsFollowing    bool
}

type ProfileReader interface {
	FindByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
	FindCardByID(ctx context.Context, userID, viewerID uuid.UUID) (ProfileCard, error)
	FindCardByTelegramID(ctx context.Context, telegramID int64, viewerID uuid.UUID) (ProfileCard, error)
}
