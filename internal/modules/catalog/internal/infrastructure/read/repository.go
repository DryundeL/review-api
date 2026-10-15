package read

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/catalog/internal/application/ports"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (ports.MediaObject, error) {
	if r == nil || r.pool == nil || id == uuid.Nil {
		return ports.MediaObject{}, contract.ErrNotFound
	}
	const query = `
		SELECT id, title, type, release_year, poster_url, release_status,
			external_source, external_id, created_by
		FROM media_objects
		WHERE id = $1
	`
	item, err := scanItem(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.MediaObject{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.MediaObject{}, fmt.Errorf("get media object: %w", err)
	}
	return item, nil
}

func (r *Repository) Search(ctx context.Context, text, typeCode string, limit int) ([]ports.MediaObject, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	const query = `
		SELECT id, title, type, release_year, poster_url, release_status,
			external_source, external_id, created_by
		FROM media_objects
		WHERE ($1::text = '' OR type = $1)
		AND ($2::text = '' OR title ILIKE '%' || $2 || '%' ESCAPE '\')
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, query, typeCode, escapeLike(text), limit)
	if err != nil {
		return nil, fmt.Errorf("search media objects: %w", err)
	}
	defer rows.Close()
	out := make([]ports.MediaObject, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan media object: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search media objects: %w", err)
	}
	return out, nil
}

func scanItem(row pgx.Row) (ports.MediaObject, error) {
	var item ports.MediaObject
	var year *int
	var poster, source, externalID *string
	var createdBy *uuid.UUID
	err := row.Scan(
		&item.ID, &item.Title, &item.Type, &year, &poster, &item.ReleaseStatus,
		&source, &externalID, &createdBy,
	)
	if err != nil {
		return ports.MediaObject{}, err
	}
	item.ReleaseYear = year
	if poster != nil {
		item.PosterURL = *poster
	}
	if source != nil {
		item.ExternalSource = *source
	}
	if externalID != nil {
		item.ExternalID = *externalID
	}
	item.CreatedBy = createdBy
	return item, nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
