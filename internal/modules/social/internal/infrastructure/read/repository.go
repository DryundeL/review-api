package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/ports"
)

const maxListLimit = 50

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListFollowing(ctx context.Context, followerID uuid.UUID, limit int) ([]ports.Follow, error) {
	return r.list(ctx, `
		SELECT follower_id, followee_id, created_at
		FROM follows
		WHERE follower_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, followerID, limit)
}

func (r *Repository) ListFollowers(ctx context.Context, followeeID uuid.UUID, limit int) ([]ports.Follow, error) {
	return r.list(ctx, `
		SELECT follower_id, followee_id, created_at
		FROM follows
		WHERE followee_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, followeeID, limit)
}

func (r *Repository) list(ctx context.Context, query string, userID uuid.UUID, limit int) ([]ports.Follow, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	if userID == uuid.Nil {
		return nil, contract.ErrInvalid
	}
	if limit <= 0 || limit > maxListLimit {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list follows: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Follow, 0)
	for rows.Next() {
		var item ports.Follow
		if err := rows.Scan(&item.FollowerID, &item.FolloweeID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan follow: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
