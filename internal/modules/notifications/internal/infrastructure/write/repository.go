package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/notifications/contract"
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

func (r *Repository) TypeEnabled(ctx context.Context, userID uuid.UUID, name string) (bool, error) {
	const query = `
		SELECT CASE $2
			WHEN 'like' THEN COALESCE(s.like_enabled, TRUE)
			WHEN 'follow' THEN COALESCE(s.follow_enabled, TRUE)
			ELSE FALSE
		END
		FROM (SELECT 1) AS _
		LEFT JOIN user_notification_settings s ON s.user_id = $1
	`
	var enabled bool
	if err := r.queryRow(ctx, query, userID, name).Scan(&enabled); err != nil {
		return false, fmt.Errorf("type enabled: %w", err)
	}
	return enabled, nil
}

func (r *Repository) TypeMeta(ctx context.Context, name string) (ports.TypeMeta, error) {
	const query = `SELECT name, title, text FROM notification_types WHERE name = $1`
	var meta ports.TypeMeta
	err := r.queryRow(ctx, query, name).Scan(&meta.Name, &meta.Title, &meta.Text)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.TypeMeta{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.TypeMeta{}, fmt.Errorf("type meta: %w", err)
	}
	return meta, nil
}

func (r *Repository) Insert(ctx context.Context, item ports.Notification) error {
	const query = `
		INSERT INTO notifications (
			id, user_id, type_name, title, body, actor_id, review_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if err := r.exec(ctx, query,
		item.ID, item.UserID, item.TypeName, item.Title, item.Body,
		item.ActorID, item.ReviewID, item.CreatedAt.UTC(),
	); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *Repository) ListInbox(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ports.Notification, error) {
	const query = `
		SELECT id, user_id, type_name, title, body, actor_id, review_id, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Notification, 0, limit)
	for rows.Next() {
		var item ports.Notification
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.TypeName, &item.Title, &item.Body,
			&item.ActorID, &item.ReviewID, &item.ReadAt, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("list notifications: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	const query = `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`
	var n int64
	if err := r.queryRow(ctx, query, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("unread count: %w", err)
	}
	return n, nil
}

func (r *Repository) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	const query = `
		UPDATE notifications
		SET read_at = COALESCE(read_at, NOW())
		WHERE id = $1 AND user_id = $2
	`
	tag, err := r.execTag(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	const query = `
		UPDATE notifications
		SET read_at = NOW()
		WHERE user_id = $1 AND read_at IS NULL
	`
	if err := r.exec(ctx, query, userID); err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.execTag(ctx, sql, args...)
	return err
}

func (r *Repository) execTag(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	return r.pool.Exec(ctx, sql, args...)
}

func (r *Repository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	return r.pool.Query(ctx, sql, args...)
}

func (r *Repository) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}
