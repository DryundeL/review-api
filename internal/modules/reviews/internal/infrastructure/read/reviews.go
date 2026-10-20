package read

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

const maxListLimit = 50

// spoilersExistSQL is true when any block body has a ||...|| pair.
const spoilersExistSQL = `
	(SELECT EXISTS (
		SELECT 1 FROM review_blocks b
		WHERE b.review_id = r.id AND b.body ~ '\|\|.*\|\|'
	)) AS has_spoilers
`

const mediaObjectSelectSQL = `mo.id, mo.title, mo.type, mo.poster_url`

// ReviewRepository reads reviews with counters (no aggregate hydration).
type ReviewRepository struct {
	pool *pgxpool.Pool
}

func NewReviewRepository(pool *pgxpool.Pool) *ReviewRepository {
	return &ReviewRepository{pool: pool}
}

func (r *ReviewRepository) Get(ctx context.Context, id, viewerID uuid.UUID) (ports.Review, error) {
	if r == nil || r.pool == nil || id == uuid.Nil {
		return ports.Review{}, contract.ErrNotFound
	}
	const query = `
		SELECT r.id, r.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			($2 <> '00000000-0000-0000-0000-000000000000'::uuid
				AND EXISTS (
					SELECT 1 FROM follows f
					WHERE f.follower_id = $2 AND f.followee_id = r.author_id
				)
			) AS is_following,
			COALESCE(gp.equipped_avatar_frame, ''), COALESCE(gp.equipped_backdrop, ''),
			` + mediaObjectSelectSQL + `, r.title, r.rating, r.status,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count,
			(SELECT v.is_like FROM review_votes v WHERE v.review_id = r.id AND v.user_id = $2) AS viewer_is_like
		FROM reviews r
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		INNER JOIN users u ON u.id = r.author_id
		LEFT JOIN gamification_profiles gp ON gp.user_id = r.author_id
		WHERE r.id = $1
	`
	item, err := scanReview(r.pool.QueryRow(ctx, query, id, viewerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Review{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.Review{}, fmt.Errorf("get review: %w", err)
	}
	blocks, err := r.loadBlocks(ctx, id)
	if err != nil {
		return ports.Review{}, err
	}
	item.Blocks = blocks
	item.HasSpoilers = blocksHaveSpoilers(blocks)
	return item, nil
}

func (r *ReviewRepository) ListByMedia(ctx context.Context, mediaObjectID, viewerID uuid.UUID, limit int) ([]ports.Review, error) {
	if mediaObjectID == uuid.Nil {
		return nil, contract.FieldError{Field: "mediaObjectId"}
	}
	return r.list(ctx, `
		SELECT r.id, r.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			COALESCE(gp.equipped_avatar_frame, ''), COALESCE(gp.equipped_backdrop, ''),
			`+mediaObjectSelectSQL+`, r.title, r.rating, r.status,
			`+spoilersExistSQL+`,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count,
			(SELECT v.is_like FROM review_votes v WHERE v.review_id = r.id AND v.user_id = $2) AS viewer_is_like
		FROM reviews r
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		INNER JOIN users u ON u.id = r.author_id
		LEFT JOIN gamification_profiles gp ON gp.user_id = r.author_id
		WHERE r.media_object_id = $1 AND r.status = 'published'
		ORDER BY r.created_at DESC
		LIMIT $3
	`, mediaObjectID, viewerID, limit)
}

func (r *ReviewRepository) ListByAuthor(ctx context.Context, authorID, viewerID uuid.UUID, publishedOnly bool, limit int) ([]ports.Review, error) {
	if authorID == uuid.Nil {
		return nil, contract.FieldError{Field: "authorId"}
	}
	statusFilter := ""
	if publishedOnly {
		statusFilter = " AND r.status = 'published'"
	}
	return r.list(ctx, `
		SELECT r.id, r.author_id, COALESCE(u.username, ''), COALESCE(u.photo_url, ''),
			COALESCE(gp.equipped_avatar_frame, ''), COALESCE(gp.equipped_backdrop, ''),
			`+mediaObjectSelectSQL+`, r.title, r.rating, r.status,
			`+spoilersExistSQL+`,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count,
			(SELECT v.is_like FROM review_votes v WHERE v.review_id = r.id AND v.user_id = $2) AS viewer_is_like
		FROM reviews r
		INNER JOIN media_objects mo ON mo.id = r.media_object_id
		INNER JOIN users u ON u.id = r.author_id
		LEFT JOIN gamification_profiles gp ON gp.user_id = r.author_id
		WHERE r.author_id = $1`+statusFilter+`
		ORDER BY r.created_at DESC
		LIMIT $3
	`, authorID, viewerID, limit)
}

func (r *ReviewRepository) list(ctx context.Context, query string, filterID, viewerID uuid.UUID, limit int) ([]ports.Review, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	if limit <= 0 || limit > maxListLimit {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, query, filterID, viewerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Review, 0)
	for rows.Next() {
		item, err := scanListReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	return out, nil
}

func (r *ReviewRepository) loadBlocks(ctx context.Context, reviewID uuid.UUID) ([]ports.Block, error) {
	const query = `
		SELECT id, position, title, body
		FROM review_blocks
		WHERE review_id = $1
		ORDER BY position
	`
	rows, err := r.pool.Query(ctx, query, reviewID)
	if err != nil {
		return nil, fmt.Errorf("list review blocks: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Block, 0)
	for rows.Next() {
		var block ports.Block
		if err := rows.Scan(&block.ID, &block.Position, &block.Title, &block.Body); err != nil {
			return nil, fmt.Errorf("scan review block: %w", err)
		}
		out = append(out, block)
	}
	return out, rows.Err()
}

func scanReview(row pgx.Row) (ports.Review, error) {
	var item ports.Review
	var poster *string
	err := row.Scan(
		&item.ID, &item.AuthorID, &item.Author.Username, &item.Author.PhotoURL, &item.Author.IsFollowing,
		&item.Author.AvatarFrame, &item.Author.Backdrop,
		&item.MediaObject.ID, &item.MediaObject.Title, &item.MediaObject.Type, &poster,
		&item.Title, &item.Rating, &item.Status,
		&item.LikesCount, &item.DislikesCount, &item.ViewsCount,
		&item.ViewerIsLike,
	)
	if err != nil {
		return ports.Review{}, err
	}
	item.Author.ID = item.AuthorID
	if poster != nil {
		item.MediaObject.PosterURL = *poster
	}
	return item, nil
}

func scanListReview(row pgx.Row) (ports.Review, error) {
	var item ports.Review
	var poster *string
	err := row.Scan(
		&item.ID, &item.AuthorID, &item.Author.Username, &item.Author.PhotoURL,
		&item.Author.AvatarFrame, &item.Author.Backdrop,
		&item.MediaObject.ID, &item.MediaObject.Title, &item.MediaObject.Type, &poster,
		&item.Title, &item.Rating, &item.Status,
		&item.HasSpoilers,
		&item.LikesCount, &item.DislikesCount, &item.ViewsCount,
		&item.ViewerIsLike,
	)
	if err != nil {
		return ports.Review{}, err
	}
	item.Author.ID = item.AuthorID
	if poster != nil {
		item.MediaObject.PosterURL = *poster
	}
	return item, nil
}

func blocksHaveSpoilers(blocks []ports.Block) bool {
	for _, b := range blocks {
		if hasSpoilerMarkup(b.Body) {
			return true
		}
	}
	return false
}

// hasSpoilerMarkup is true when body contains at least one ||...|| pair.
func hasSpoilerMarkup(body string) bool {
	i := strings.Index(body, "||")
	if i < 0 {
		return false
	}
	return strings.Contains(body[i+2:], "||")
}
