package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/search/contract"
	"review-api/internal/modules/search/internal/application/ports"
)

// Repository is a read-side search over users/reviews/catalog.
// ponytail: ILIKE %q% / GROUP BY tops; pg_trgm or counters if latency shows up
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) TopMedia(ctx context.Context, limit int) ([]ports.MediaTrend, error) {
	if r == nil || r.pool == nil || limit <= 0 {
		return nil, contract.ErrInvalid
	}
	const q = `
		SELECT mo.id, mo.title, mo.type, mo.poster_url, COUNT(*)::bigint AS reviews_count
		FROM reviews r
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		WHERE r.status = 'published'
		GROUP BY mo.id
		ORDER BY reviews_count DESC, mo.title ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("top media: %w", err)
	}
	defer rows.Close()

	out := make([]ports.MediaTrend, 0, limit)
	for rows.Next() {
		var item ports.MediaTrend
		var poster *string
		if err := rows.Scan(&item.ID, &item.Title, &item.Type, &poster, &item.ReviewsCount); err != nil {
			return nil, fmt.Errorf("scan top media: %w", err)
		}
		if poster != nil {
			item.PosterURL = *poster
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) TopAuthors(ctx context.Context, viewerID uuid.UUID, limit int) ([]ports.AuthorTrend, error) {
	if r == nil || r.pool == nil || limit <= 0 {
		return nil, contract.ErrInvalid
	}
	const q = `
		SELECT u.id,
			COALESCE(u.username, ''),
			u.first_name,
			COALESCE(u.photo_url, ''),
			COUNT(f.follower_id)::bigint AS followers_count,
			(
				$1::uuid <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (
					SELECT 1 FROM follows me
					WHERE me.follower_id = $1 AND me.followee_id = u.id
				)
			) AS is_following
		FROM users u
		LEFT JOIN follows f ON f.followee_id = u.id
		GROUP BY u.id
		ORDER BY followers_count DESC, COALESCE(u.username, '') ASC, u.id ASC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, viewerID, limit)
	if err != nil {
		return nil, fmt.Errorf("top authors: %w", err)
	}
	defer rows.Close()

	out := make([]ports.AuthorTrend, 0, limit)
	for rows.Next() {
		var item ports.AuthorTrend
		if err := rows.Scan(
			&item.ID, &item.Username, &item.FirstName, &item.PhotoURL, &item.FollowersCount, &item.IsFollowing,
		); err != nil {
			return nil, fmt.Errorf("scan top author: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) TopTypes(ctx context.Context) ([]ports.TypeTrend, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	const q = `
		SELECT rt.code, rt.label, COUNT(r.id)::bigint AS reviews_count
		FROM review_types rt
		LEFT JOIN media_objects mo ON mo.type = rt.code
		LEFT JOIN reviews r ON r.media_object_id = mo.id AND r.status = 'published'
		WHERE rt.is_active
		GROUP BY rt.code, rt.label, rt.sort_order
		ORDER BY reviews_count DESC, rt.sort_order ASC
	`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("top types: %w", err)
	}
	defer rows.Close()

	out := make([]ports.TypeTrend, 0)
	for rows.Next() {
		var item ports.TypeTrend
		if err := rows.Scan(&item.Code, &item.Label, &item.ReviewsCount); err != nil {
			return nil, fmt.Errorf("scan top type: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) SearchUsers(ctx context.Context, q string, viewerID uuid.UUID, limit int) ([]ports.UserHit, error) {
	if r == nil || r.pool == nil || q == "" || limit <= 0 {
		return nil, contract.ErrInvalid
	}
	like := escapeLike(q)
	const sql = `
		SELECT u.id,
			COALESCE(u.username, ''),
			u.first_name,
			COALESCE(u.photo_url, ''),
			(SELECT COUNT(*) FROM follows f WHERE f.followee_id = u.id),
			(
				$3::uuid <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (
					SELECT 1 FROM follows me
					WHERE me.follower_id = $3 AND me.followee_id = u.id
				)
			)
		FROM users u
		WHERE u.username ILIKE '%' || $2 || '%' ESCAPE '\'
		ORDER BY
			CASE WHEN lower(u.username) = lower($1) THEN 0 ELSE 1 END,
			u.username ASC
		LIMIT $4
	`
	rows, err := r.pool.Query(ctx, sql, q, like, viewerID, limit)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	defer rows.Close()

	out := make([]ports.UserHit, 0, limit)
	for rows.Next() {
		var item ports.UserHit
		if err := rows.Scan(
			&item.ID, &item.Username, &item.FirstName, &item.PhotoURL, &item.FollowersCount, &item.IsFollowing,
		); err != nil {
			return nil, fmt.Errorf("scan user hit: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) SearchReviews(ctx context.Context, q string, limit int) ([]ports.ReviewHit, error) {
	if r == nil || r.pool == nil || q == "" || limit <= 0 {
		return nil, contract.ErrInvalid
	}
	like := escapeLike(q)
	const sql = `
		SELECT r.id, r.title, r.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			mo.id, mo.title, mo.type, mo.poster_url,
			r.rating, r.likes_count, r.views_count
		FROM reviews r
		INNER JOIN users u ON u.id = r.author_id
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		WHERE r.status = 'published'
			AND r.title ILIKE '%' || $1 || '%' ESCAPE '\'
		ORDER BY r.likes_count DESC, r.created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, sql, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search reviews: %w", err)
	}
	defer rows.Close()

	out := make([]ports.ReviewHit, 0, limit)
	for rows.Next() {
		var item ports.ReviewHit
		var poster *string
		if err := rows.Scan(
			&item.ID, &item.Title, &item.AuthorID, &item.Username, &item.PhotoURL,
			&item.MediaObject.ID, &item.MediaObject.Title, &item.MediaObject.Type, &poster,
			&item.Rating, &item.LikesCount, &item.ViewsCount,
		); err != nil {
			return nil, fmt.Errorf("scan review hit: %w", err)
		}
		if poster != nil {
			item.MediaObject.PosterURL = *poster
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) SearchMedia(ctx context.Context, q string, limit int) ([]ports.MediaHit, error) {
	if r == nil || r.pool == nil || q == "" || limit <= 0 {
		return nil, contract.ErrInvalid
	}
	like := escapeLike(q)
	const sql = `
		SELECT id, title, type, poster_url
		FROM media_objects
		WHERE title ILIKE '%' || $1 || '%' ESCAPE '\'
			OR original_title ILIKE '%' || $1 || '%' ESCAPE '\'
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, sql, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search media: %w", err)
	}
	defer rows.Close()

	out := make([]ports.MediaHit, 0, limit)
	for rows.Next() {
		var item ports.MediaHit
		var poster *string
		if err := rows.Scan(&item.ID, &item.Title, &item.Type, &poster); err != nil {
			return nil, fmt.Errorf("scan media hit: %w", err)
		}
		if poster != nil {
			item.PosterURL = *poster
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
