package write

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/media/contract"
	"review-api/internal/modules/media/internal/application/ports"
	"review-api/internal/platform/transaction"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, item ports.ImageRecord) error {
	if r == nil || r.pool == nil {
		return contract.ErrInvalid
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO media_images (id, owner_id, storage_key, content_type, size_bytes, width, height, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		item.ID, item.OwnerID, item.StorageKey, item.ContentType, item.SizeBytes, item.Width, item.Height, item.CreatedAt)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23514") {
		return contract.ErrInvalid
	}
	return fmt.Errorf("insert image: %w", err)
}

func (r *Repository) LockUnattached(ctx context.Context, cutoff time.Time, limit int) ([]ports.ImageRecord, error) {
	tx, ok := transaction.TxFromContext(ctx)
	if !ok || limit < 1 {
		return nil, contract.ErrInvalid
	}
	rows, err := tx.Query(ctx, `
		SELECT id, owner_id, storage_key, content_type, size_bytes, width, height, created_at
		FROM media_images
		WHERE attached_at IS NULL AND created_at < $1
		ORDER BY created_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED`, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("lock unattached images: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ports.ImageRecord, error) {
		var it ports.ImageRecord
		err := row.Scan(&it.ID, &it.OwnerID, &it.StorageKey, &it.ContentType, &it.SizeBytes, &it.Width, &it.Height, &it.CreatedAt)
		return it, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan unattached images: %w", err)
	}
	return items, nil
}

func (r *Repository) DeleteByIDs(ctx context.Context, ids []uuid.UUID) error {
	tx, ok := transaction.TxFromContext(ctx)
	if !ok {
		return contract.ErrInvalid
	}
	if len(ids) == 0 {
		return nil
	}
	// attached_at IS NULL guards against deleting a row that was attached after it was read.
	if _, err := tx.Exec(ctx, `DELETE FROM media_images WHERE id = ANY($1) AND attached_at IS NULL`, ids); err != nil {
		return fmt.Errorf("delete images: %w", err)
	}
	return nil
}
