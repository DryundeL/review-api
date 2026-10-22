package ports

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/users/internal/domain"
)

type UserRepository interface {
	Upsert(ctx context.Context, user domain.User) error
	EnsureSettings(ctx context.Context, telegramID int64, language string) error
	AllowNotifications(ctx context.Context, telegramID int64) (bool, error)
	AllowNotificationsByUserID(ctx context.Context, userID uuid.UUID) (bool, error)
	SetAllowNotifications(ctx context.Context, telegramID int64, allow bool) error
	GetSettings(ctx context.Context, userID uuid.UUID) (domain.Settings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, language *string, allow *bool) (domain.Settings, error)
}

type EmailRepository interface {
	SetEmail(ctx context.Context, telegramID int64, email string) error
	FindTelegramIDByEmail(ctx context.Context, email string) (int64, error)
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
	FindByID(ctx context.Context, userID uuid.UUID) (domain.User, error)
	FindCardByID(ctx context.Context, userID, viewerID uuid.UUID) (ProfileCard, error)
	FindCardByTelegramID(ctx context.Context, telegramID int64, viewerID uuid.UUID) (ProfileCard, error)
}
