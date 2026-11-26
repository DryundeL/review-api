package read

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
)

// OpenReports is the moderation queue, oldest first; it uses list_reports_open_idx.
func (r *Repository) OpenReports(ctx context.Context, after *ports.PageCursor, limit int) ([]ports.ReportCard, error) {
	if r == nil || r.pool == nil || limit < 1 {
		return nil, contract.ErrInvalid
	}
	const q = `
		SELECT rp.id, rp.template_id, rp.result_id, rp.reporter_id, rp.reason_code, rp.status, rp.created_at,
			COALESCE(t.title, r.title, ''), COALESCE(t.author_id, r.author_id),
			COALESCE(t.moderation_hidden_at, r.moderation_hidden_at) IS NOT NULL,
			COALESCE(t.deleted_at, r.deleted_at) IS NOT NULL
		FROM list_reports rp
		LEFT JOIN list_templates t ON t.id = rp.template_id
		LEFT JOIN list_results r ON r.id = rp.result_id
		WHERE rp.status = 'open'
			AND ($1::timestamptz IS NULL OR (rp.created_at, rp.id) > ($1, $2))
		ORDER BY rp.created_at, rp.id
		LIMIT $3`
	var (
		afterT  any
		afterID = uuid.Nil
	)
	if after != nil {
		afterT, afterID = after.T, after.ID
	}
	rows, err := r.pool.Query(ctx, q, afterT, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list open list reports: %w", err)
	}
	cards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ports.ReportCard, error) {
		var (
			c                    ports.ReportCard
			templateID, resultID *uuid.UUID
		)
		err := row.Scan(&c.ID, &templateID, &resultID, &c.ReporterID, &c.ReasonCode, &c.Status, &c.CreatedAt,
			&c.TargetTitle, &c.TargetAuthorID, &c.TargetHidden, &c.TargetDeleted)
		if templateID != nil {
			c.Target, c.TargetID = ports.TargetTemplate, *templateID
		} else if resultID != nil {
			c.Target, c.TargetID = ports.TargetResult, *resultID
		}
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("list open list reports: %w", err)
	}
	return cards, nil
}
