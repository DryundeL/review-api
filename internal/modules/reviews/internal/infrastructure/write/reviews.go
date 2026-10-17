package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

// ReviewRepository persists reviews, blocks, votes, and views.
type ReviewRepository struct {
	pool *pgxpool.Pool
}

func NewReviewRepository(pool *pgxpool.Pool) *ReviewRepository {
	return &ReviewRepository{pool: pool}
}

func (r *ReviewRepository) Create(ctx context.Context, item domain.Review) error {
	const insertReview = `
		INSERT INTO reviews (id, author_id, media_object_id, title, rating, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if err := r.exec(ctx, insertReview,
		item.ID, item.AuthorID, item.MediaObjectID, item.Title, item.Rating, item.Status,
	); err != nil {
		return mapReviewWriteError(err)
	}
	return r.insertBlocks(ctx, item.ID, item.Blocks)
}

func (r *ReviewRepository) Replace(ctx context.Context, item domain.Review) error {
	const updateReview = `
		UPDATE reviews
		SET title = $2, rating = $3, status = $4, updated_at = NOW()
		WHERE id = $1
	`
	tag, err := r.execTag(ctx, updateReview, item.ID, item.Title, item.Rating, item.Status)
	if err != nil {
		return mapReviewWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	if err := r.exec(ctx, `DELETE FROM review_blocks WHERE review_id = $1`, item.ID); err != nil {
		return fmt.Errorf("delete review blocks: %w", err)
	}
	return r.insertBlocks(ctx, item.ID, item.Blocks)
}

func (r *ReviewRepository) Get(ctx context.Context, id uuid.UUID) (domain.Review, error) {
	if id == uuid.Nil {
		return domain.Review{}, contract.ErrNotFound
	}
	const query = `
		SELECT id, author_id, media_object_id, title, rating, status
		FROM reviews
		WHERE id = $1
	`
	var item domain.Review
	err := r.queryRow(ctx, query, id).Scan(
		&item.ID, &item.AuthorID, &item.MediaObjectID, &item.Title, &item.Rating, &item.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Review{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.Review{}, fmt.Errorf("get review: %w", err)
	}
	blocks, err := r.loadBlocks(ctx, id)
	if err != nil {
		return domain.Review{}, err
	}
	item.Blocks = blocks
	return item, nil
}

func (r *ReviewRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.execTag(ctx, `DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		return mapReviewWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *ReviewRepository) UpsertVote(ctx context.Context, reviewID, userID uuid.UUID, isLike bool) error {
	var prevLike bool
	err := r.queryRow(ctx, `
		SELECT is_like FROM review_votes WHERE review_id = $1 AND user_id = $2
	`, reviewID, userID).Scan(&prevLike)
	var prev *bool
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		prev = nil
	case err != nil:
		return fmt.Errorf("get vote: %w", err)
	default:
		prev = &prevLike
	}

	const upsert = `
		INSERT INTO review_votes (review_id, user_id, is_like)
		VALUES ($1, $2, $3)
		ON CONFLICT (review_id, user_id) DO UPDATE
		SET is_like = EXCLUDED.is_like, updated_at = NOW()
	`
	if err := r.exec(ctx, upsert, reviewID, userID, isLike); err != nil {
		return mapReviewWriteError(err)
	}

	delta := voteLikesDelta(prev, isLike)
	if delta == 0 {
		return nil
	}
	const bump = `
		UPDATE reviews
		SET likes_count = GREATEST(0, likes_count + $2)
		WHERE id = $1
	`
	if err := r.exec(ctx, bump, reviewID, delta); err != nil {
		return mapReviewWriteError(err)
	}
	return nil
}

func voteLikesDelta(prev *bool, isLike bool) int {
	if prev == nil {
		if isLike {
			return 1
		}
		return 0
	}
	if *prev == isLike {
		return 0
	}
	if isLike {
		return 1
	}
	return -1
}

func (r *ReviewRepository) InsertView(ctx context.Context, reviewID, userID uuid.UUID) (bool, error) {
	const query = `
		INSERT INTO review_views (review_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (review_id, user_id) DO NOTHING
	`
	tag, err := r.execTag(ctx, query, reviewID, userID)
	if err != nil {
		return false, mapReviewWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	const bump = `
		UPDATE reviews
		SET views_count = views_count + 1
		WHERE id = $1
	`
	if err := r.exec(ctx, bump, reviewID); err != nil {
		return false, mapReviewWriteError(err)
	}
	return true, nil
}

func (r *ReviewRepository) insertBlocks(ctx context.Context, reviewID uuid.UUID, blocks []domain.Block) error {
	const insert = `
		INSERT INTO review_blocks (id, review_id, position, title, body)
		VALUES ($1, $2, $3, $4, $5)
	`
	for _, block := range blocks {
		if err := r.exec(ctx, insert, block.ID, reviewID, block.Position, block.Title, block.Body); err != nil {
			return mapReviewWriteError(err)
		}
	}
	return nil
}

func (r *ReviewRepository) loadBlocks(ctx context.Context, reviewID uuid.UUID) ([]domain.Block, error) {
	const query = `
		SELECT id, position, title, body
		FROM review_blocks
		WHERE review_id = $1
		ORDER BY position
	`
	rows, err := r.query(ctx, query, reviewID)
	if err != nil {
		return nil, fmt.Errorf("list review blocks: %w", err)
	}
	defer rows.Close()
	out := make([]domain.Block, 0)
	for rows.Next() {
		var block domain.Block
		if err := rows.Scan(&block.ID, &block.Position, &block.Title, &block.Body); err != nil {
			return nil, fmt.Errorf("scan review block: %w", err)
		}
		out = append(out, block)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.execTag(ctx, sql, args...)
	return err
}

func (r *ReviewRepository) execTag(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	if r == nil || r.pool == nil {
		return pgconn.CommandTag{}, contract.ErrInvalid
	}
	return r.pool.Exec(ctx, sql, args...)
}

func (r *ReviewRepository) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}

func (r *ReviewRepository) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Query(ctx, sql, args...)
	}
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	return r.pool.Query(ctx, sql, args...)
}

func mapReviewWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		switch pgErr.ConstraintName {
		case "reviews_media_object_id_fkey":
			return contract.FieldError{Field: "mediaObjectId"}
		case "reviews_author_id_fkey", "review_votes_user_id_fkey", "review_views_user_id_fkey":
			return contract.FieldError{Field: "userId"}
		case "review_votes_review_id_fkey", "review_views_review_id_fkey":
			return contract.ErrNotFound
		default:
			return contract.FieldError{Field: "body"}
		}
	case "23505":
		return contract.ErrConflict
	case "23514":
		return contract.FieldError{Field: "body"}
	default:
		return err
	}
}
