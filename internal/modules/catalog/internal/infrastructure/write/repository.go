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

func (r *Repository) Save(ctx context.Context, item domain.MediaObject) error {
	const insert = `
		INSERT INTO media_objects (
			id, title, original_title, type, info, poster_url,
			external_source, external_id, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`
	var original, poster any
	if item.OriginalTitle != "" {
		original = item.OriginalTitle
	}
	if item.PosterURL != "" {
		poster = item.PosterURL
	}
	var source, externalID any
	if item.ExternalSource != "" {
		source = item.ExternalSource
		externalID = item.ExternalID
	}
	var createdBy any
	if item.CreatedBy != nil {
		createdBy = *item.CreatedBy
	}
	if err := r.exec(ctx, insert,
		item.ID, item.Title, original, item.Type, item.Info, poster,
		source, externalID, createdBy,
	); err != nil {
		return mapWriteError(err)
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
		return contract.FieldError{Field: "type", Index: -1}
	case "23505":
		return contract.FieldError{Field: "external", Index: -1}
	case "23514":
		return contract.FieldError{Field: "body", Index: -1}
	default:
		return fmt.Errorf("save media object: %w", err)
	}
}
