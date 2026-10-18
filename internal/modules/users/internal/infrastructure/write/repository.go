package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/users/contract"
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
			id, telegram_id, username, first_name, last_name, language_code, is_premium, photo_url, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $9
		)
		ON CONFLICT (telegram_id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			language_code = EXCLUDED.language_code,
			is_premium = EXCLUDED.is_premium,
			photo_url = COALESCE(EXCLUDED.photo_url, users.photo_url),
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
		nullString(user.PhotoURL),
		user.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func (r *Repository) EnsureSettings(ctx context.Context, telegramID int64, language string) error {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return contract.ErrNotFound
	}
	const query = `
		INSERT INTO user_settings (user_id, language, notifications_muted)
		SELECT id, $2, FALSE
		FROM users
		WHERE telegram_id = $1
		ON CONFLICT (user_id) DO NOTHING
	`
	tag, err := r.execTag(ctx, query, telegramID, language)
	if err != nil {
		return fmt.Errorf("ensure settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Conflict or missing user: distinguish via existence check.
		var exists bool
		err = r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id = $1)`, telegramID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("ensure settings: %w", err)
		}
		if !exists {
			return contract.ErrNotFound
		}
	}
	return nil
}

func (r *Repository) NotificationsMuted(ctx context.Context, telegramID int64) (bool, error) {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return false, contract.ErrNotFound
	}
	const query = `
		SELECT s.notifications_muted
		FROM user_settings s
		JOIN users u ON u.id = s.user_id
		WHERE u.telegram_id = $1
	`
	var muted bool
	err := r.queryRow(ctx, query, telegramID).Scan(&muted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, contract.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("notifications muted: %w", err)
	}
	return muted, nil
}

func (r *Repository) SetNotificationsMuted(ctx context.Context, telegramID int64, muted bool) error {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return contract.ErrNotFound
	}
	const query = `
		UPDATE user_settings s
		SET notifications_muted = $2, updated_at = NOW()
		FROM users u
		WHERE s.user_id = u.id AND u.telegram_id = $1
	`
	tag, err := r.execTag(ctx, query, telegramID, muted)
	if err != nil {
		return fmt.Errorf("set notifications muted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *Repository) GetSettings(ctx context.Context, userID uuid.UUID) (domain.Settings, error) {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return domain.Settings{}, contract.ErrNotFound
	}
	const query = `
		SELECT user_id, language, notifications_muted
		FROM user_settings
		WHERE user_id = $1
	`
	var s domain.Settings
	err := r.queryRow(ctx, query, userID).Scan(&s.UserID, &s.Language, &s.NotificationsMuted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return s, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, userID uuid.UUID, language *string, muted *bool) (domain.Settings, error) {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return domain.Settings{}, contract.ErrNotFound
	}
	const query = `
		UPDATE user_settings
		SET language = COALESCE($2, language),
		    notifications_muted = COALESCE($3, notifications_muted),
		    updated_at = NOW()
		WHERE user_id = $1
		RETURNING user_id, language, notifications_muted
	`
	var s domain.Settings
	err := r.queryRow(ctx, query, userID, language, muted).Scan(&s.UserID, &s.Language, &s.NotificationsMuted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("update settings: %w", err)
	}
	return s, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
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

func (r *Repository) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}
