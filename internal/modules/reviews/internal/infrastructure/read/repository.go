package read

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, activeOnly bool) ([]ports.Type, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrNotFound
	}
	const query = `
		SELECT code, label, sort_order, is_active
		FROM review_types
		WHERE NOT $1::bool OR is_active
		ORDER BY sort_order, code
	`
	rows, err := r.pool.Query(ctx, query, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list review types: %w", err)
	}
	defer rows.Close()

	out := make([]ports.Type, 0)
	for rows.Next() {
		var item ports.Type
		if err := rows.Scan(&item.Code, &item.Label, &item.SortOrder, &item.IsActive); err != nil {
			return nil, fmt.Errorf("scan review type: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list review types: %w", err)
	}
	return out, nil
}

func (r *Repository) RequireActive(ctx context.Context, codes []string) error {
	if r == nil || r.pool == nil || len(codes) == 0 {
		return contract.FieldError{Field: "type"}
	}
	rows, err := r.pool.Query(ctx, `
		SELECT code
		FROM review_types
		WHERE code = ANY($1::text[]) AND is_active
	`, codes)
	if err != nil {
		return fmt.Errorf("check review types: %w", err)
	}
	defer rows.Close()
	found := map[string]struct{}{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return fmt.Errorf("scan review type: %w", err)
		}
		found[code] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("check review types: %w", err)
	}
	if len(found) != len(codes) {
		return contract.FieldError{Field: "type"}
	}
	return nil
}
