package read

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindByTelegramID(ctx context.Context, telegramID int64) (domain.User, error) {
	card, err := r.FindCardByTelegramID(ctx, telegramID, uuid.Nil)
	if err != nil {
		return domain.User{}, err
	}
	return card.User, nil
}

func (r *Repository) FindByID(ctx context.Context, userID uuid.UUID) (domain.User, error) {
	card, err := r.FindCardByID(ctx, userID, uuid.Nil)
	if err != nil {
		return domain.User{}, err
	}
	return card.User, nil
}

func (r *Repository) FindCardByID(ctx context.Context, userID, viewerID uuid.UUID) (ports.ProfileCard, error) {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return ports.ProfileCard{}, contract.ErrNotFound
	}
	const query = `
		SELECT u.id, u.telegram_id,
			COALESCE(u.username, ''),
			u.first_name,
			COALESCE(u.last_name, ''),
			COALESCE(u.language_code, ''),
			u.is_premium,
			COALESCE(u.photo_url, ''),
			COALESCE(u.email, ''),
			CASE
				WHEN EXISTS (SELECT 1 FROM follows WHERE follower_id = $2 AND followee_id = u.id) THEN 'following'
				WHEN EXISTS (SELECT 1 FROM follow_requests WHERE follower_id = $2 AND followee_id = u.id) THEN 'pending'
				ELSE 'none'
			END,
			COALESCE((SELECT s.is_profile_private FROM user_settings s WHERE s.user_id = u.id), FALSE),
			(SELECT COUNT(*) FROM follows f WHERE f.followee_id = u.id),
			(SELECT COUNT(*) FROM follows f WHERE f.follower_id = u.id),
			EXISTS (
				SELECT 1 FROM follows f
				WHERE f.follower_id = $2 AND f.followee_id = u.id AND $2 <> '00000000-0000-0000-0000-000000000000'::uuid
			)
		FROM users u
		WHERE u.id = $1
	`
	return r.scanCard(r.pool.QueryRow(ctx, query, userID, viewerID))
}

func (r *Repository) FindCardByTelegramID(ctx context.Context, telegramID int64, viewerID uuid.UUID) (ports.ProfileCard, error) {
	if r == nil || r.pool == nil || telegramID <= 0 {
		return ports.ProfileCard{}, contract.ErrNotFound
	}
	const query = `
		SELECT u.id, u.telegram_id,
			COALESCE(u.username, ''),
			u.first_name,
			COALESCE(u.last_name, ''),
			COALESCE(u.language_code, ''),
			u.is_premium,
			COALESCE(u.photo_url, ''),
			COALESCE(u.email, ''),
			CASE
				WHEN EXISTS (SELECT 1 FROM follows WHERE follower_id = $2 AND followee_id = u.id) THEN 'following'
				WHEN EXISTS (SELECT 1 FROM follow_requests WHERE follower_id = $2 AND followee_id = u.id) THEN 'pending'
				ELSE 'none'
			END,
			COALESCE((SELECT s.is_profile_private FROM user_settings s WHERE s.user_id = u.id), FALSE),
			(SELECT COUNT(*) FROM follows f WHERE f.followee_id = u.id),
			(SELECT COUNT(*) FROM follows f WHERE f.follower_id = u.id),
			EXISTS (
				SELECT 1 FROM follows f
				WHERE f.follower_id = $2 AND f.followee_id = u.id AND $2 <> '00000000-0000-0000-0000-000000000000'::uuid
			)
		FROM users u
		WHERE u.telegram_id = $1
	`
	return r.scanCard(r.pool.QueryRow(ctx, query, telegramID, viewerID))
}

func (r *Repository) scanCard(row pgx.Row) (ports.ProfileCard, error) {
	var card ports.ProfileCard
	err := row.Scan(
		&card.User.ID,
		&card.User.TelegramID,
		&card.User.Username,
		&card.User.FirstName,
		&card.User.LastName,
		&card.User.LanguageCode,
		&card.User.IsPremium,
		&card.User.PhotoURL,
		&card.User.Email,
		&card.FollowStatus,
		&card.IsProfilePrivate,
		&card.FollowersCount,
		&card.FollowingCount,
		&card.IsFollowing,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ProfileCard{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.ProfileCard{}, fmt.Errorf("find profile card: %w", err)
	}
	return card, nil
}
