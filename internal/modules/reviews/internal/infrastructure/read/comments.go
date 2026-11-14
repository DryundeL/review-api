package read

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/platform/transaction"
)

const commentSelect = `SELECT c.id,c.review_id,c.author_id,c.parent_id,c.body,c.created_at,
    COALESCE(u.username,''), COALESCE(u.photo_url,''),
    EXISTS(SELECT 1 FROM review_comments child WHERE child.review_id=c.review_id AND child.parent_id=c.id)
    FROM review_comments c JOIN users u ON u.id=c.author_id
    JOIN reviews r ON r.id=c.review_id AND r.status='published' `

func scanComment(row pgx.Row) (ports.Comment, error) {
	var c ports.Comment
	err := row.Scan(&c.ID, &c.ReviewID, &c.AuthorID, &c.ParentID, &c.Body, &c.CreatedAt, &c.Author.Username, &c.Author.PhotoURL, &c.HasReplies)
	c.Author.ID = c.AuthorID
	return c, err
}

func (r *ReviewRepository) commentRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}

func (r *ReviewRepository) GetComment(ctx context.Context, reviewID, commentID uuid.UUID) (ports.Comment, error) {
	c, err := scanComment(r.commentRow(ctx, commentSelect+`WHERE c.review_id=$1 AND c.id=$2`, reviewID, commentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Comment{}, contract.ErrNotFound
	}
	return c, err
}

func (r *ReviewRepository) ListComments(ctx context.Context, reviewID uuid.UUID, parentID *uuid.UUID, after *ports.CommentPosition, limit int) ([]ports.Comment, error) {
	if limit < 1 || limit > 51 {
		return nil, contract.FieldError{Field: "limit"}
	}
	var exists bool
	err := r.commentRow(ctx, `SELECT EXISTS(SELECT 1 FROM reviews WHERE id=$1 AND status='published')`, reviewID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, contract.ErrNotFound
	}
	if parentID != nil {
		if _, err := r.GetComment(ctx, reviewID, *parentID); err != nil {
			return nil, err
		}
	}
	args := []any{reviewID, limit}
	filter := `WHERE c.review_id=$1 AND c.parent_id IS NULL`
	if parentID != nil {
		args = append(args, *parentID)
		filter = `WHERE c.review_id=$1 AND c.parent_id=$3`
	}
	if after != nil {
		n := len(args)
		args = append(args, after.CreatedAt, after.ID)
		// Parameter numbers depend only on the fixed query shape, never on user input.
		if n == 2 {
			filter += ` AND (c.created_at,c.id)>($3,$4)`
		} else {
			filter += ` AND (c.created_at,c.id)>($4,$5)`
		}
	}
	sql := commentSelect + filter + ` ORDER BY c.created_at,c.id LIMIT $2`
	var rows pgx.Rows
	if tx, ok := transaction.TxFromContext(ctx); ok {
		rows, err = tx.Query(ctx, sql, args...)
	} else {
		rows, err = r.pool.Query(ctx, sql, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.Comment, 0, limit)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ReviewRepository) CommentNotificationRef(ctx context.Context, commentID uuid.UUID) (contract.CommentNotificationRef, error) {
	var ref contract.CommentNotificationRef
	err := r.commentRow(ctx, `SELECT c.review_id,r.title,c.author_id,c.parent_id,COALESCE(p.author_id,r.author_id)
        FROM review_comments c JOIN reviews r ON r.id=c.review_id AND r.status='published'
        LEFT JOIN review_comments p ON p.review_id=c.review_id AND p.id=c.parent_id WHERE c.id=$1`, commentID).
		Scan(&ref.ReviewID, &ref.ReviewTitle, &ref.AuthorID, &ref.ParentID, &ref.RecipientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ref, contract.ErrNotFound
	}
	return ref, err
}
