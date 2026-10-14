package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/domain"
	"review-api/internal/platform/transaction"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Save(ctx context.Context, work domain.Work) error {
	const insertWork = `
		INSERT INTO works (
			id, title, primary_type, release_year, cover_url, release_status, origin, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`
	var cover any
	if work.CoverURL != "" {
		cover = work.CoverURL
	}
	if err := r.exec(ctx, insertWork,
		work.ID, work.Title, work.PrimaryType, work.ReleaseYear, cover, work.ReleaseStatus, work.Origin, work.CreatedBy,
	); err != nil {
		return mapWriteError(err)
	}
	const insertType = `INSERT INTO work_types (work_id, type_code) VALUES ($1, $2)`
	for _, code := range work.Types {
		if err := r.exec(ctx, insertType, work.ID, code); err != nil {
			return mapWriteError(err)
		}
	}
	return nil
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

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		if pgErr.ConstraintName == "works_created_by_fkey" {
			return contract.ErrAuthorNotFound
		}
		return contract.FieldError{Field: "type"}
	case "23514":
		return contract.FieldError{Field: "body"}
	default:
		return fmt.Errorf("save work: %w", err)
	}
}
