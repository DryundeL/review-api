package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

const maxSort = 10000

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, item domain.ReviewType) error {
	const query = `
		INSERT INTO review_types (code, label, sort_order, is_active)
		VALUES ($1, $2, $3, $4)
	`
	if err := r.exec(ctx, query, item.Code, item.Label, item.SortOrder, item.IsActive); err != nil {
		return mapWriteError(err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, code string, label *string, sortOrder *int, active *bool) (domain.ReviewType, error) {
	const query = `
		UPDATE review_types
		SET label = COALESCE($2, label),
			sort_order = COALESCE($3, sort_order),
			is_active = COALESCE($4, is_active),
			updated_at = NOW()
		WHERE code = $1
		RETURNING code, label, sort_order, is_active
	`
	var item domain.ReviewType
	err := r.queryRow(ctx, query, code, label, sortOrder, active).Scan(
		&item.Code, &item.Label, &item.SortOrder, &item.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReviewType{}, contract.ErrNotFound
	}
	if err != nil {
		return domain.ReviewType{}, mapWriteError(err)
	}
	return item, nil
}

func (r *Repository) NextSort(ctx context.Context) (int, error) {
	const query = `SELECT COALESCE(MAX(sort_order), 0) + 10 FROM review_types`
	var next int
	if err := r.queryRow(ctx, query).Scan(&next); err != nil {
		return 0, fmt.Errorf("next review type sort: %w", err)
	}
	if next > maxSort {
		return 0, contract.FieldError{Field: "sortOrder"}
	}
	return next, nil
}

func (r *Repository) exec(ctx context.Context, sql string, args ...any) error {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		_, err := tx.Exec(ctx, sql, args...)
		return err
	}
	if r == nil || r.pool == nil {
		return contract.ErrInvalid
	}
	_, err := r.pool.Exec(ctx, sql, args...)
	return err
}

func (r *Repository) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.TxFromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return r.pool.QueryRow(ctx, sql, args...)
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return contract.ErrConflict
		case "23514":
			return contract.FieldError{Field: "body"}
		}
	}
	return err
}
