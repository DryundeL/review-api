package read

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

const maxListLimit = 50

// ReviewRepository reads reviews with counters (no aggregate hydration).
type ReviewRepository struct {
	pool *pgxpool.Pool
}

func NewReviewRepository(pool *pgxpool.Pool) *ReviewRepository {
	return &ReviewRepository{pool: pool}
}

func (r *ReviewRepository) Get(ctx context.Context, id uuid.UUID) (ports.Review, error) {
	if r == nil || r.pool == nil || id == uuid.Nil {
		return ports.Review{}, contract.ErrNotFound
	}
	const query = `
		SELECT id, author_id, media_object_id, title, rating, status,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count
		FROM reviews r
		WHERE id = $1
	`
	item, err := scanReview(r.pool.QueryRow(ctx, query, id))
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
	return item, nil
}

func (r *ReviewRepository) ListByMedia(ctx context.Context, mediaObjectID uuid.UUID, limit int) ([]ports.Review, error) {
	if mediaObjectID == uuid.Nil {
		return nil, contract.FieldError{Field: "mediaObjectId"}
	}
	return r.list(ctx, `
		SELECT id, author_id, media_object_id, title, rating, status,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count
		FROM reviews r
		WHERE media_object_id = $1 AND status = 'published'
		ORDER BY created_at DESC
		LIMIT $2
	`, mediaObjectID, limit)
}

func (r *ReviewRepository) ListByAuthor(ctx context.Context, authorID uuid.UUID, publishedOnly bool, limit int) ([]ports.Review, error) {
	if authorID == uuid.Nil {
		return nil, contract.FieldError{Field: "authorId"}
	}
	if publishedOnly {
		return r.list(ctx, `
			SELECT id, author_id, media_object_id, title, rating, status,
				(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
				(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
				(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count
			FROM reviews r
			WHERE author_id = $1 AND status = 'published'
			ORDER BY created_at DESC
			LIMIT $2
		`, authorID, limit)
	}
	return r.list(ctx, `
		SELECT id, author_id, media_object_id, title, rating, status,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND v.is_like) AS likes_count,
			(SELECT COUNT(*) FROM review_votes v WHERE v.review_id = r.id AND NOT v.is_like) AS dislikes_count,
			(SELECT COUNT(*) FROM review_views v WHERE v.review_id = r.id) AS views_count
		FROM reviews r
		WHERE author_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, authorID, limit)
}

func (r *ReviewRepository) list(ctx context.Context, query string, filterID uuid.UUID, limit int) ([]ports.Review, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	if limit <= 0 || limit > maxListLimit {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, query, filterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Review, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		item, err := scanReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan review: %w", err)
		}
		out = append(out, item)
		ids = append(ids, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	if len(ids) == 0 {
		return out, nil
	}
	blocksByReview, err := r.loadBlocksForReviews(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Blocks = blocksByReview[out[i].ID]
		if out[i].Blocks == nil {
			out[i].Blocks = []ports.Block{}
		}
	}
	return out, nil
}

func (r *ReviewRepository) loadBlocks(ctx context.Context, reviewID uuid.UUID) ([]ports.Block, error) {
	m, err := r.loadBlocksForReviews(ctx, []uuid.UUID{reviewID})
	if err != nil {
		return nil, err
	}
	blocks := m[reviewID]
	if blocks == nil {
		return []ports.Block{}, nil
	}
	return blocks, nil
}

func (r *ReviewRepository) loadBlocksForReviews(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]ports.Block, error) {
	const query = `
		SELECT id, review_id, position, title, body
		FROM review_blocks
		WHERE review_id = ANY($1)
		ORDER BY review_id, position
	`
	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("list review blocks: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]ports.Block, len(ids))
	for rows.Next() {
		var block ports.Block
		var reviewID uuid.UUID
		if err := rows.Scan(&block.ID, &reviewID, &block.Position, &block.Title, &block.Body); err != nil {
			return nil, fmt.Errorf("scan review block: %w", err)
		}
		out[reviewID] = append(out[reviewID], block)
	}
	return out, rows.Err()
}

func scanReview(row pgx.Row) (ports.Review, error) {
	var item ports.Review
	err := row.Scan(
		&item.ID, &item.AuthorID, &item.MediaObjectID, &item.Title, &item.Rating, &item.Status,
		&item.LikesCount, &item.DislikesCount, &item.ViewsCount,
	)
	return item, err
}
