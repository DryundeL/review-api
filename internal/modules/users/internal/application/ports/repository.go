package ports

import (
	"context"

	"review-api/internal/modules/users/internal/domain"
)

type UserRepository interface {
	Upsert(ctx context.Context, user domain.User) error
}

type ProfileReader interface {
	FindByTelegramID(ctx context.Context, telegramID int64) (domain.User, error)
}
