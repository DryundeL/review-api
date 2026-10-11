package write

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/users/internal/domain"
	"review-api/internal/platform/transaction"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Upsert(ctx context.Context, user domain.User) error {
	const query = `
		INSERT INTO users (
			id, telegram_id, username, first_name, last_name, language_code, is_premium, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $8
		)
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			language_code = EXCLUDED.language_code,
			is_premium = EXCLUDED.is_premium,
			updated_at = EXCLUDED.updated_at
	`
	err := r.exec(ctx, query,
		user.ID,
		user.TelegramID,
		nullString(user.Username),
		user.FirstName,
		nullString(user.LastName),
		nullString(user.LanguageCode),
		user.IsPremium,
		user.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) error {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		_, err := tx.Exec(ctx, sql, args...)
		return err
	}
	_, err := r.pool.Exec(ctx, sql, args...)
	return err
}
