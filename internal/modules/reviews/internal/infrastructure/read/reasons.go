package read

import (
	"context"
	"fmt"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

func (r *Repository) ListActive(ctx context.Context) ([]ports.Reason, error) {
	if r == nil || r.pool == nil {
		return nil, contract.ErrInvalid
	}
	const query = `
		SELECT code, label, sort_order
		FROM report_reasons
		WHERE is_active
		ORDER BY sort_order, code
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list report reasons: %w", err)
	}
	defer rows.Close()

	out := make([]ports.Reason, 0)
	for rows.Next() {
		var item ports.Reason
		if err := rows.Scan(&item.Code, &item.Label, &item.SortOrder); err != nil {
			return nil, fmt.Errorf("scan report reason: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list report reasons: %w", err)
	}
	return out, nil
}

func (r *Repository) IsActive(ctx context.Context, code string) (bool, error) {
	if r == nil || r.pool == nil || code == "" {
		return false, nil
	}
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM report_reasons WHERE code = $1 AND is_active
		)
	`
	var ok bool
	if err := r.pool.QueryRow(ctx, query, code).Scan(&ok); err != nil {
		return false, fmt.Errorf("check report reason: %w", err)
	}
	return ok, nil
}
