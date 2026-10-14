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

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (ports.Work, error) {
	if r == nil || r.pool == nil || id == uuid.Nil {
		return ports.Work{}, contract.ErrNotFound
	}
	const query = `
		SELECT w.id, w.title, w.primary_type, w.release_year, w.cover_url, w.release_status, w.origin, w.created_by,
			ARRAY(SELECT wt.type_code FROM work_types wt WHERE wt.work_id = w.id ORDER BY wt.type_code)
		FROM works w
		WHERE w.id = $1
	`
	work, err := scanWork(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.Work{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.Work{}, fmt.Errorf("get work: %w", err)
	}
	return work, nil
}

func (r *Repository) Search(ctx context.Context, text, typeCode string, limit int) ([]ports.Work, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	const query = `
		SELECT w.id, w.title, w.primary_type, w.release_year, w.cover_url, w.release_status, w.origin, w.created_by,
			ARRAY(SELECT wt.type_code FROM work_types wt WHERE wt.work_id = w.id ORDER BY wt.type_code)
		FROM works w
		WHERE ($1::text = '' OR EXISTS (
			SELECT 1 FROM work_types wt WHERE wt.work_id = w.id AND wt.type_code = $1
		))
		AND ($2::text = '' OR w.title ILIKE '%' || $2 || '%' ESCAPE '\')
		ORDER BY w.created_at DESC
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, query, typeCode, escapeLike(text), limit)
	if err != nil {
		return nil, fmt.Errorf("search works: %w", err)
	}
	defer rows.Close()
	out := make([]ports.Work, 0)
	for rows.Next() {
		work, err := scanWork(rows)
		if err != nil {
			return nil, fmt.Errorf("scan work: %w", err)
		}
		out = append(out, work)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search works: %w", err)
	}
	return out, nil
}

func scanWork(row pgx.Row) (ports.Work, error) {
	var work ports.Work
	var year *int
	var cover *string
	var types []string
	err := row.Scan(
		&work.ID, &work.Title, &work.PrimaryType, &year, &cover, &work.ReleaseStatus, &work.Origin, &work.CreatedBy, &types,
	)
	if err != nil {
		return ports.Work{}, err
	}
	work.ReleaseYear = year
	if cover != nil {
		work.CoverURL = *cover
	}
	work.Types = orderTypes(work.PrimaryType, types)
	return work, nil
}

func orderTypes(primary string, types []string) []string {
	if len(types) == 0 {
		if primary == "" {
			return []string{}
		}
		return []string{primary}
	}
	rest := make([]string, 0, len(types))
	hasPrimary := false
	for _, code := range types {
		if code == primary {
			hasPrimary = true
			continue
		}
		rest = append(rest, code)
	}
	if !hasPrimary {
		return append([]string{primary}, rest...)
	}
	return append([]string{primary}, rest...)
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}
