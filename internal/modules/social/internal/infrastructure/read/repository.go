package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/ports"
)

const (
	maxListLimit  = 50
	maxListOffset = 10_000
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UserExists(ctx context.Context, userID uuid.UUID) (bool, error) {
	if r == nil || r.pool == nil || userID == uuid.Nil {
		return false, nil
	}
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&ok); err != nil {
		return false, fmt.Errorf("user exists: %w", err)
	}
	return ok, nil
}

func (r *Repository) ListFollowing(ctx context.Context, subjectID, viewerID uuid.UUID, limit, offset int) ([]ports.FollowUser, error) {
	return r.list(ctx, `
		SELECT u.id,
			COALESCE(u.username, ''),
			COALESCE(u.photo_url, ''),
			(
				$2::uuid <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (
					SELECT 1 FROM follows me
					WHERE me.follower_id = $2 AND me.followee_id = u.id
				)
			) AS is_following,
			f.created_at
		FROM follows f
		INNER JOIN users u ON u.id = f.followee_id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC
		LIMIT $3 OFFSET $4
	`, subjectID, viewerID, limit, offset)
}

func (r *Repository) ListFollowers(ctx context.Context, subjectID, viewerID uuid.UUID, limit, offset int) ([]ports.FollowUser, error) {
	return r.list(ctx, `
		SELECT u.id,
			COALESCE(u.username, ''),
			COALESCE(u.photo_url, ''),
			(
				$2::uuid <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (
					SELECT 1 FROM follows me
					WHERE me.follower_id = $2 AND me.followee_id = u.id
				)
			) AS is_following,
			f.created_at
		FROM follows f
		INNER JOIN users u ON u.id = f.follower_id
		WHERE f.followee_id = $1
		ORDER BY f.created_at DESC
		LIMIT $3 OFFSET $4
	`, subjectID, viewerID, limit, offset)
}

func (r *Repository) list(ctx context.Context, query string, subjectID, viewerID uuid.UUID, limit, offset int) ([]ports.FollowUser, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	if subjectID == uuid.Nil {
		return nil, contract.ErrInvalid
	}
	if limit <= 0 || limit > maxListLimit {
		limit = 20
	}
	if offset < 0 || offset > maxListOffset {
		return nil, contract.FieldError{Field: "offset"}
	}
	rows, err := r.pool.Query(ctx, query, subjectID, viewerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list follows: %w", err)
	}
	defer rows.Close()
	out := make([]ports.FollowUser, 0)
	for rows.Next() {
		var item ports.FollowUser
		if err := rows.Scan(
			&item.UserID, &item.Username, &item.PhotoURL, &item.IsFollowing, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan follow: %w", err)
		}
		if viewerID != uuid.Nil && item.UserID == viewerID {
			item.IsFollowing = false
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
