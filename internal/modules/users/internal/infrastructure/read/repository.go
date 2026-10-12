package read

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindByTelegramID(ctx context.Context, telegramID int64) (domain.User, error) {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return domain.User{}, contract.ErrNotFound
	}
	const query = `
		SELECT id, telegram_id,
			COALESCE(username, ''),
			first_name,
			COALESCE(last_name, ''),
			COALESCE(language_code, ''),
			is_premium
		FROM users
		WHERE telegram_id = $1
	`
	var user domain.User
	err := r.pool.QueryRow(ctx, query, telegramID).Scan(
		&user.ID,
		&user.TelegramID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
		&user.LanguageCode,
		&user.IsPremium,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}
	return user, nil
}
