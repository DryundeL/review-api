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
		INSERT INTO user_settings (user_id, language, allow_notifications)
		SELECT id, $2, TRUE
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

func (r *Repository) AllowNotifications(ctx context.Context, telegramID int64) (bool, error) {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return false, contract.ErrNotFound
	}
	const query = `
		SELECT s.allow_notifications
		FROM user_settings s
		JOIN users u ON u.id = s.user_id
		WHERE u.telegram_id = $1
	`
	var allow bool
	err := r.queryRow(ctx, query, telegramID).Scan(&allow)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, contract.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("allow notifications: %w", err)
	}
	return allow, nil
}

func (r *Repository) AllowNotificationsByUserID(ctx context.Context, userID uuid.UUID) (bool, error) {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return false, contract.ErrNotFound
	}
	const query = `SELECT allow_notifications FROM user_settings WHERE user_id = $1`
	var allow bool
	err := r.queryRow(ctx, query, userID).Scan(&allow)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil // no settings row → allow by default
	}
	if err != nil {
		return false, fmt.Errorf("allow notifications by user: %w", err)
	}
	return allow, nil
}

func (r *Repository) SetAllowNotifications(ctx context.Context, telegramID int64, allow bool) error {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return contract.ErrNotFound
	}
	const query = `
		UPDATE user_settings s
		SET allow_notifications = $2, updated_at = NOW()
		FROM users u
		WHERE s.user_id = u.id AND u.telegram_id = $1
	`
	tag, err := r.execTag(ctx, query, telegramID, allow)
	if err != nil {
		return fmt.Errorf("set allow notifications: %w", err)
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
		SELECT user_id, language, allow_notifications, is_profile_private
		FROM user_settings
		WHERE user_id = $1
	`
	var s domain.Settings
	err := r.queryRow(ctx, query, userID).Scan(&s.UserID, &s.Language, &s.AllowNotifications, &s.IsProfilePrivate)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("get settings: %w", err)
	}
	return s, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, userID uuid.UUID, language *string, allow *bool, private *bool) (domain.Settings, error) {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return domain.Settings{}, contract.ErrNotFound
	}
	const query = `
		WITH target AS (SELECT id FROM users WHERE id = $1 FOR NO KEY UPDATE)
		UPDATE user_settings
		SET language = COALESCE($2, language),
		    allow_notifications = COALESCE($3, allow_notifications),
		    is_profile_private = COALESCE($4, is_profile_private),
		    updated_at = NOW()
		FROM target
		WHERE user_id = target.id
		RETURNING user_id, language, allow_notifications, is_profile_private
	`
	var s domain.Settings
	err := r.queryRow(ctx, query, userID, language, allow, private).Scan(&s.UserID, &s.Language, &s.AllowNotifications, &s.IsProfilePrivate)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("update settings: %w", err)
	}
	return s, nil
}

func (r *Repository) SetEmail(ctx context.Context, telegramID int64, email string) error {
	if r == nil || r.pool == nil || telegramID <= 0 || email == "" {
		return contract.ErrNotFound
	}
	tag, err := r.execTag(ctx, `UPDATE users SET email = $2, updated_at = NOW() WHERE telegram_id = $1`, telegramID, email)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return contract.ErrEmailConflict
	}
	if err != nil {
		return fmt.Errorf("set user email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *Repository) FindTelegramIDByEmail(ctx context.Context, email string) (int64, error) {
	if r == nil || r.pool == nil || email == "" {
		return 0, contract.ErrNotFound
	}
	var telegramID int64
	err := r.queryRow(ctx, `SELECT telegram_id FROM users WHERE email = $1`, email).Scan(&telegramID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, contract.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("find user by email: %w", err)
	}
	return telegramID, nil
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
