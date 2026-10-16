package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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

func (r *Repository) Create(ctx context.Context, item domain.Follow) error {
	const query = `
		INSERT INTO follows (follower_id, followee_id)
		VALUES ($1, $2)
	`
	if err := r.exec(ctx, query, item.FollowerID, item.FolloweeID); err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, followerID, followeeID uuid.UUID) error {
	const query = `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	tag, err := r.execTag(ctx, query, followerID, followeeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
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
