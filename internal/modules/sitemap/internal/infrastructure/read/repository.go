package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/sitemap/internal/application/query"
)

// Repository is a read-only projection across catalog, reviews and profiles.
type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) List(ctx context.Context, entity string, after uuid.UUID, limit int) ([]uuid.UUID, error) {
	var sql string
	switch entity {
	case "media-objects":
		sql = `SELECT id FROM media_objects WHERE id > $1 ORDER BY id LIMIT $2`
	case "reviews":
		sql = `SELECT id FROM reviews WHERE status = 'published' AND id > $1 ORDER BY id LIMIT $2`
	case "authors":
		sql = `SELECT u.id FROM users u
		LEFT JOIN user_settings s ON s.user_id = u.id
		WHERE NOT COALESCE(s.is_profile_private, FALSE) AND u.id > $1
		ORDER BY u.id LIMIT $2`
	default:
		return nil, query.ErrInvalid
	}
	if limit < 1 || limit > 1001 {
		return nil, query.ErrInvalid
	}
	rows, err := r.pool.Query(ctx, sql, after, limit)
	if err != nil {
		return nil, fmt.Errorf("list sitemap IDs: %w", err)
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0, limit)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan sitemap ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sitemap IDs: %w", err)
	}
	return ids, nil
}
