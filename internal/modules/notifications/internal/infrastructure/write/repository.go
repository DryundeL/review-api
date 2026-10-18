package write

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/notifications/internal/application/ports"
	"review-api/internal/platform/transaction"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) EnsureDefaults(ctx context.Context, userID uuid.UUID) error {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return nil
	}
	const query = `
		INSERT INTO user_notification_settings (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`
	if err := r.exec(ctx, query, userID); err != nil {
		return fmt.Errorf("ensure notification defaults: %w", err)
	}
	return nil
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID) ([]ports.PrefItem, error) {
	const query = `
		SELECT t.name, t.title, t.text,
			CASE t.name
				WHEN 'like' THEN COALESCE(s.like_enabled, TRUE)
				WHEN 'follow' THEN COALESCE(s.follow_enabled, TRUE)
				ELSE TRUE
			END
		FROM notification_types t
		LEFT JOIN user_notification_settings s ON s.user_id = $1
		ORDER BY t.name
	`
	rows, err := r.query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list notification settings: %w", err)
	}
	defer rows.Close()

	out := make([]ports.PrefItem, 0, 2)
	for rows.Next() {
		var item ports.PrefItem
		if err := rows.Scan(&item.Name, &item.Title, &item.Text, &item.Enabled); err != nil {
			return nil, fmt.Errorf("list notification settings: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notification settings: %w", err)
	}
	return out, nil
}

func (r *Repository) Patch(ctx context.Context, userID uuid.UUID, like, follow *bool) error {
	const query = `
		UPDATE user_notification_settings
		SET like_enabled = COALESCE($2, like_enabled),
		    follow_enabled = COALESCE($3, follow_enabled)
		WHERE user_id = $1
	`
	if err := r.exec(ctx, query, userID, like, follow); err != nil {
		return fmt.Errorf("patch notification settings: %w", err)
	}
	return nil
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) error {
	var err error
	if tx, ok := transaction.TxFromContext(ctx); ok {
		_, err = tx.Exec(ctx, sql, args...)
	} else {
		_, err = r.pool.Exec(ctx, sql, args...)
	}
	return err
}

func (r *Repository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return r.pool.Query(ctx, sql, args...)
}
