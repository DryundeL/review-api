package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/domain"
	"review-api/internal/platform/transaction"
)

// The template row is share-locked too, so that hiding the template is ordered before or after.
const lockOpenResultSQL = `
	SELECT r.author_id
	FROM list_results r
	JOIN list_templates t ON t.id = r.template_id
	WHERE r.id = $1 AND r.status = 'published' AND r.deleted_at IS NULL
		AND r.moderation_hidden_at IS NULL AND t.moderation_hidden_at IS NULL`

func (r *Repository) LockOpenResult(ctx context.Context, id uuid.UUID, exclusive bool) (uuid.UUID, error) {
	if _, ok := transaction.TxFromContext(ctx); !ok {
		return uuid.Nil, contract.ErrInvalid
	}
	if id == uuid.Nil {
		return uuid.Nil, contract.ErrNotFound
	}
	lock := ` FOR SHARE OF r, t`
	if exclusive {
		lock = ` FOR NO KEY UPDATE OF r FOR SHARE OF t`
	}
	var authorID uuid.UUID
	err := r.queryRow(ctx, lockOpenResultSQL+lock, id).Scan(&authorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, contract.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lock list result: %w", err)
	}
	return authorID, nil
}

// UpsertVote relies on the exclusive lock of LockOpenResult: votes on one result are serialized,
// so the previous vote read here cannot change before the counters are adjusted.
func (r *Repository) UpsertVote(ctx context.Context, resultID, userID uuid.UUID, isLike bool) error {
	var prevLike bool
	var prev *bool
	err := r.queryRow(ctx, `SELECT is_like FROM list_result_votes WHERE result_id = $1 AND user_id = $2`,
		resultID, userID).Scan(&prevLike)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return fmt.Errorf("get list result vote: %w", err)
	default:
		prev = &prevLike
	}
	if prev != nil && *prev == isLike {
		return nil
	}
	if _, err := r.exec(ctx, `
		INSERT INTO list_result_votes (result_id, user_id, is_like) VALUES ($1, $2, $3)
		ON CONFLICT (result_id, user_id) DO UPDATE SET is_like = EXCLUDED.is_like, updated_at = NOW()`,
		resultID, userID, isLike); err != nil {
		return mapReactionError(err)
	}
	likes, dislikes := voteDeltas(prev, isLike)
	if _, err := r.exec(ctx, `
		UPDATE list_results
		SET likes_count = GREATEST(0, likes_count + $2), dislikes_count = GREATEST(0, dislikes_count + $3)
		WHERE id = $1`, resultID, likes, dislikes); err != nil {
		return fmt.Errorf("update list result votes: %w", err)
	}
	return nil
}

// voteDeltas returns the change of the like and dislike counters when prev becomes isLike.
func voteDeltas(prev *bool, isLike bool) (likes, dislikes int) {
	switch {
	case prev == nil && isLike:
		return 1, 0
	case prev == nil:
		return 0, 1
	case *prev == isLike:
		return 0, 0
	case isLike:
		return 1, -1
	default:
		return -1, 1
	}
}

func (r *Repository) InsertView(ctx context.Context, resultID, userID uuid.UUID) error {
	tag, err := r.exec(ctx, `
		INSERT INTO list_result_views (result_id, user_id) VALUES ($1, $2)
		ON CONFLICT (result_id, user_id) DO NOTHING`, resultID, userID)
	if err != nil {
		return mapReactionError(err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if _, err := r.exec(ctx, `UPDATE list_results SET views_count = views_count + 1 WHERE id = $1`, resultID); err != nil {
		return fmt.Errorf("update list result views: %w", err)
	}
	return nil
}

func (r *Repository) CommentExists(ctx context.Context, resultID, commentID uuid.UUID) (bool, error) {
	var exists bool
	err := r.queryRow(ctx, `SELECT EXISTS (SELECT 1 FROM list_result_comments WHERE result_id = $1 AND id = $2)`,
		resultID, commentID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check list comment: %w", err)
	}
	return exists, nil
}

func (r *Repository) InsertComment(ctx context.Context, c domain.Comment) error {
	if _, err := r.exec(ctx, `
		INSERT INTO list_result_comments (id, result_id, author_id, parent_id, body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.ResultID, c.AuthorID, c.ParentID, c.Body, c.CreatedAt); err != nil {
		return mapReactionError(err)
	}
	return nil
}

// mapReactionError: a user or parent comment removed in a race is the same 404 as a missing one.
func mapReactionError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return contract.ErrNotFound
	}
	return err
}
