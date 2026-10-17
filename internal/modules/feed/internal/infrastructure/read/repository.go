package read

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/feed/contract"
	"review-api/internal/modules/feed/internal/application/ports"
)

const spoilersExistSQL = `
	(SELECT EXISTS (
		SELECT 1 FROM review_blocks b
		WHERE b.review_id = r.id AND b.body ~ '\|\|.*\|\|'
	)) AS has_spoilers
`

// Repository reads the published feed via keyset pagination.
// ponytail: direct read of reviews/follows; materialize when write volume or following fan-out hurts
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, p ports.ListParams) ([]ports.Item, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	if p.Limit <= 0 {
		return nil, contract.ErrInvalid
	}

	var b strings.Builder
	args := make([]any, 0, 8)
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	viewerArg := arg(p.ViewerID)

	b.WriteString(`
		SELECT r.id, r.author_id, COALESCE(u.username, ''),
			( `)
	b.WriteString(viewerArg)
	b.WriteString(` <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (
					SELECT 1 FROM follows f
					WHERE f.follower_id = `)
	b.WriteString(viewerArg)
	b.WriteString(` AND f.followee_id = r.author_id
				)
			) AS is_following,
			mo.id, mo.title, mo.type, mo.poster_url,
			r.title, r.rating, `)
	b.WriteString(spoilersExistSQL)
	b.WriteString(`,
			r.likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			r.views_count,
			r.created_at
		FROM reviews r
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		INNER JOIN users u ON u.id = r.author_id
		WHERE r.status = 'published'
	`)

	if p.Mode == ports.ModeFollowing {
		if p.ViewerID == uuid.Nil {
			return nil, contract.ErrUnauthorized
		}
		b.WriteString(` AND EXISTS (
			SELECT 1 FROM follows f
			WHERE f.follower_id = `)
		b.WriteString(viewerArg)
		b.WriteString(` AND f.followee_id = r.author_id
		)`)
	}

	if p.Type != "" {
		b.WriteString(` AND mo.type = `)
		b.WriteString(arg(p.Type))
	}

	if p.Cursor != nil {
		switch p.Sort {
		case ports.SortViews:
			b.WriteString(` AND (r.views_count, r.created_at, r.id) < (`)
			b.WriteString(arg(p.Cursor.V))
			b.WriteString(`, `)
			b.WriteString(arg(p.Cursor.T))
			b.WriteString(`, `)
			b.WriteString(arg(p.Cursor.ID))
			b.WriteString(`)`)
		case ports.SortLikes:
			b.WriteString(` AND (r.likes_count, r.created_at, r.id) < (`)
			b.WriteString(arg(p.Cursor.V))
			b.WriteString(`, `)
			b.WriteString(arg(p.Cursor.T))
			b.WriteString(`, `)
			b.WriteString(arg(p.Cursor.ID))
			b.WriteString(`)`)
		default: // new
			b.WriteString(` AND (r.created_at, r.id) < (`)
			b.WriteString(arg(p.Cursor.T))
			b.WriteString(`, `)
			b.WriteString(arg(p.Cursor.ID))
			b.WriteString(`)`)
		}
	}

	switch p.Sort {
	case ports.SortViews:
		b.WriteString(` ORDER BY r.views_count DESC, r.created_at DESC, r.id DESC`)
	case ports.SortLikes:
		b.WriteString(` ORDER BY r.likes_count DESC, r.created_at DESC, r.id DESC`)
	default:
		b.WriteString(` ORDER BY r.created_at DESC, r.id DESC`)
	}

	b.WriteString(` LIMIT `)
	b.WriteString(arg(p.Limit))

	rows, err := r.pool.Query(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	defer rows.Close()

	out := make([]ports.Item, 0, p.Limit)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan feed item: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list feed: %w", err)
	}
	return out, nil
}

func scanItem(row interface {
	Scan(dest ...any) error
}) (ports.Item, error) {
	var item ports.Item
	var poster *string
	err := row.Scan(
		&item.ID, &item.AuthorID, &item.Author.Username, &item.Author.IsFollowing,
		&item.MediaObject.ID, &item.MediaObject.Title, &item.MediaObject.Type, &poster,
		&item.Title, &item.Rating,
		&item.HasSpoilers,
		&item.LikesCount, &item.DislikesCount, &item.ViewsCount,
		&item.CreatedAt,
	)
	if err != nil {
		return ports.Item{}, err
	}
	item.Author.ID = item.AuthorID
	if poster != nil {
		item.MediaObject.PosterURL = *poster
	}
	return item, nil
}
