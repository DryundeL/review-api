package write

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
)

func (r *Repository) InsertReport(ctx context.Context, rep ports.Report) error {
	var templateID, resultID *uuid.UUID
	switch rep.Target {
	case ports.TargetTemplate:
		templateID = &rep.TargetID
	case ports.TargetResult:
		resultID = &rep.TargetID
	default:
		return contract.ErrInvalid
	}
	const q = `
		INSERT INTO list_reports (id, template_id, result_id, reporter_id, reason_code, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'open', $6)`
	_, err := r.exec(ctx, q, rep.ID, templateID, resultID, rep.ReporterID, rep.ReasonCode, rep.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505" && (pgErr.ConstraintName == "list_reports_open_template_uidx" || pgErr.ConstraintName == "list_reports_open_result_uidx"):
			return contract.ErrAlreadyReported
		case pgErr.Code == "23503" && pgErr.ConstraintName == "list_reports_reason_code_fkey":
			return contract.Field("reasonCode", contract.ReasonInvalid)
		case pgErr.Code == "23503":
			return contract.ErrNotFound
		}
	}
	if err != nil {
		return fmt.Errorf("insert list report: %w", err)
	}
	return nil
}

func (r *Repository) ResolveReport(ctx context.Context, id uuid.UUID) (ports.Report, error) {
	const q = `
		UPDATE list_reports SET status = 'resolved', resolved_at = NOW()
		WHERE id = $1 AND status = 'open'
		RETURNING id, template_id, result_id, reporter_id, reason_code, status, created_at, resolved_at`
	var (
		rep                  ports.Report
		templateID, resultID *uuid.UUID
	)
	err := r.queryRow(ctx, q, id).Scan(&rep.ID, &templateID, &resultID, &rep.ReporterID, &rep.ReasonCode,
		&rep.Status, &rep.CreatedAt, &rep.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := r.queryRow(ctx, `SELECT EXISTS (SELECT 1 FROM list_reports WHERE id = $1)`, id).Scan(&exists); err != nil {
			return ports.Report{}, fmt.Errorf("resolve list report: %w", err)
		}
		if exists {
			return ports.Report{}, contract.ErrConflict
		}
		return ports.Report{}, contract.ErrNotFound
	}
	if err != nil {
		return ports.Report{}, fmt.Errorf("resolve list report: %w", err)
	}
	if templateID != nil {
		rep.Target, rep.TargetID = ports.TargetTemplate, *templateID
	} else if resultID != nil {
		rep.Target, rep.TargetID = ports.TargetResult, *resultID
	}
	return rep, nil
}

// SetHidden does not touch updated_at: the author did not change anything.
func (r *Repository) SetHidden(ctx context.Context, target ports.ReportTarget, id uuid.UUID, hidden bool) error {
	var q string
	switch target {
	case ports.TargetTemplate:
		q = `UPDATE list_templates SET moderation_hidden_at = CASE WHEN $2 THEN COALESCE(moderation_hidden_at, NOW()) END WHERE id = $1`
	case ports.TargetResult:
		q = `UPDATE list_results SET moderation_hidden_at = CASE WHEN $2 THEN COALESCE(moderation_hidden_at, NOW()) END WHERE id = $1`
	default:
		return contract.ErrInvalid
	}
	tag, err := r.exec(ctx, q, id, hidden)
	if err != nil {
		return fmt.Errorf("set list %s moderation: %w", target, err)
	}
	if tag.RowsAffected() == 0 {
		return contract.ErrNotFound
	}
	return nil
}
