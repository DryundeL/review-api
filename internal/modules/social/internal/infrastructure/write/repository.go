package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/domain"
	"review-api/internal/platform/transaction"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// lockTarget serializes follow transitions and privacy updates on the target user.
func (r *Repository) lockTarget(ctx context.Context, id uuid.UUID) (bool, error) {
	if _, ok := transaction.TxFromContext(ctx); !ok {
		return false, contract.ErrInvalid
	}
	var target uuid.UUID
	err := r.queryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR NO KEY UPDATE`, id).Scan(&target)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, contract.ErrNotFound
	}
	if err != nil {
		return false, err
	}
	// Read after acquiring the lock to see a privacy update that just committed.
	var private bool
	err = r.queryRow(ctx, `SELECT COALESCE((SELECT is_profile_private FROM user_settings WHERE user_id=$1),FALSE)`, id).Scan(&private)
	return private, err
}

func (r *Repository) Create(ctx context.Context, item domain.Follow) (string, error) {
	private, err := r.lockTarget(ctx, item.FolloweeID)
	if err != nil {
		return "", err
	}
	var following bool
	if err := r.queryRow(ctx, `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id=$1 AND followee_id=$2)`, item.FollowerID, item.FolloweeID).Scan(&following); err != nil {
		return "", err
	}
	if following {
		return "", contract.ErrConflict
	}
	if private {
		if err := r.exec(ctx, `INSERT INTO follow_requests(follower_id,followee_id) VALUES($1,$2)`, item.FollowerID, item.FolloweeID); err != nil {
			return "", mapWriteError(err)
		}
		return "pending", nil
	}
	if err := r.exec(ctx, `INSERT INTO follows(follower_id,followee_id) VALUES($1,$2)`, item.FollowerID, item.FolloweeID); err != nil {
		return "", mapWriteError(err)
	}
	if err := r.exec(ctx, `DELETE FROM follow_requests WHERE follower_id=$1 AND followee_id=$2`, item.FollowerID, item.FolloweeID); err != nil {
		return "", err
	}
	return "following", nil
}

func (r *Repository) Approve(ctx context.Context, item domain.Follow) error {
	if err := r.DeleteRequest(ctx, item.FollowerID, item.FolloweeID); err != nil {
		return err
	}
	return mapWriteError(r.exec(ctx, `INSERT INTO follows(follower_id,followee_id) VALUES($1,$2)`, item.FollowerID, item.FolloweeID))
}

func (r *Repository) DeleteRequest(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if _, err := r.lockTarget(ctx, followeeID); err != nil {
		return err
	}
	tag, err := r.execTag(ctx, `DELETE FROM follow_requests WHERE follower_id=$1 AND followee_id=$2`, followerID, followeeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if _, err := r.lockTarget(ctx, followeeID); err != nil {
		return err
	}
	var count int
	err := r.queryRow(ctx, `WITH requests AS (DELETE FROM follow_requests WHERE follower_id=$1 AND followee_id=$2 RETURNING 1),
 follows_deleted AS (DELETE FROM follows WHERE follower_id=$1 AND followee_id=$2 RETURNING 1)
 SELECT (SELECT count(*) FROM requests)+(SELECT count(*) FROM follows_deleted)`, followerID, followeeID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return contract.ErrNotFound
	}
	return nil
}

func (r *Repository) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) error {
	_, err := r.execTag(ctx, sql, args...)
	return err
}

func (r *Repository) execTag(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	if r == nil || r.pool == nil {
		return pgconn.CommandTag{}, contract.ErrInvalid
	}
	return r.pool.Exec(ctx, sql, args...)
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		return contract.FieldError{Field: "followeeId"}
	case "23505":
		return contract.ErrConflict
	case "23514":
		return contract.FieldError{Field: "followeeId"}
	default:
		return fmt.Errorf("follow write: %w", err)
	}
}
