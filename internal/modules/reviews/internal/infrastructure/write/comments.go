package write

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

func (r *ReviewRepository) CreateComment(ctx context.Context, item domain.Comment) error {
	// The lock serializes creation with unpublishing and deleting the review.
	if _, ok := transaction.TxFromContext(ctx); !ok {
		return contract.ErrInvalid
	}
	var status string
	err := r.queryRow(ctx, `SELECT status FROM reviews WHERE id=$1 FOR SHARE`, item.ReviewID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return contract.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != domain.StatusPublished {
		return contract.ErrNotFound
	}
	if item.ParentID != nil {
		var exists bool
		err := r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM review_comments WHERE review_id=$1 AND id=$2)`, item.ReviewID, item.ParentID).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			return contract.ErrNotFound
		}
	}
	return mapReviewWriteError(r.exec(ctx, `INSERT INTO review_comments(id,review_id,author_id,parent_id,body,created_at) VALUES($1,$2,$3,$4,$5,$6)`, item.ID, item.ReviewID, item.AuthorID, item.ParentID, item.Body, item.CreatedAt))
}
