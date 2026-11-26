package command

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/lists/contract"
	"review-api/internal/modules/lists/internal/application/ports"
	"review-api/internal/modules/lists/internal/domain"
)

// ReportHandler files user reports. There is no automatic hiding: a moderator decides.
type ReportHandler struct {
	templates ports.TemplateReader
	results   ports.ResultReader
	repo      ports.ModerationWriter
	reasons   ports.ReportReasons
}

func NewReportHandler(templates ports.TemplateReader, results ports.ResultReader, repo ports.ModerationWriter, reasons ports.ReportReasons) *ReportHandler {
	return &ReportHandler{templates: templates, results: results, repo: repo, reasons: reasons}
}

// Create reports a published object the reporter can see. Objects the reporter cannot see are
// 404, the reporter's own objects are forbidden, and a second open report is a conflict.
func (h *ReportHandler) Create(ctx context.Context, target ports.ReportTarget, targetID, reporterID uuid.UUID, reasonCode string) (ports.Report, error) {
	if h == nil || h.templates == nil || h.results == nil || h.repo == nil || h.reasons == nil || reporterID == uuid.Nil {
		return ports.Report{}, contract.ErrInvalid
	}
	reasonCode = strings.TrimSpace(reasonCode)
	if reasonCode == "" {
		return ports.Report{}, contract.Field("reasonCode", contract.ReasonRequired)
	}
	if err := h.reasons.RequireActive(ctx, reasonCode); err != nil {
		return ports.Report{}, err
	}
	var authorID uuid.UUID
	var visible bool
	switch target {
	case ports.TargetTemplate:
		v, err := h.templates.Template(ctx, targetID)
		if err != nil {
			return ports.Report{}, err
		}
		authorID = v.AuthorID
		visible = v.Status == domain.StatusPublished && v.ModerationHiddenAt == nil
	case ports.TargetResult:
		v, err := h.results.Result(ctx, targetID)
		if err != nil {
			return ports.Report{}, err
		}
		authorID = v.AuthorID
		visible = v.Status == domain.StatusPublished && v.ModerationHiddenAt == nil && !v.TemplateModerated
	default:
		return ports.Report{}, contract.ErrInvalid
	}
	if authorID == reporterID {
		return ports.Report{}, contract.ErrForbidden
	}
	if !visible {
		return ports.Report{}, contract.ErrNotFound
	}
	report := ports.Report{
		ID: uuid.New(), Target: target, TargetID: targetID, ReporterID: reporterID,
		ReasonCode: reasonCode, Status: ports.ReportOpen, CreatedAt: time.Now().UTC(),
	}
	if err := h.repo.InsertReport(ctx, report); err != nil {
		return ports.Report{}, fmt.Errorf("create list report: %w", err)
	}
	return report, nil
}

// ModerationHandler holds the admin actions. Each is logged without user content.
type ModerationHandler struct {
	repo ports.ModerationWriter
}

func NewModerationHandler(repo ports.ModerationWriter) *ModerationHandler {
	return &ModerationHandler{repo: repo}
}

// Hide marks the object as hidden by a moderator. Hiding a template hides every result on it.
// Status and visibility are untouched, so Unhide restores the previous state.
func (h *ModerationHandler) Hide(ctx context.Context, target ports.ReportTarget, id uuid.UUID) error {
	return h.setHidden(ctx, target, id, true)
}

func (h *ModerationHandler) Unhide(ctx context.Context, target ports.ReportTarget, id uuid.UUID) error {
	return h.setHidden(ctx, target, id, false)
}

func (h *ModerationHandler) setHidden(ctx context.Context, target ports.ReportTarget, id uuid.UUID, hidden bool) error {
	if h == nil || h.repo == nil {
		return contract.ErrInvalid
	}
	if target != ports.TargetTemplate && target != ports.TargetResult {
		return contract.ErrInvalid
	}
	if id == uuid.Nil {
		return contract.ErrNotFound
	}
	if err := h.repo.SetHidden(ctx, target, id, hidden); err != nil {
		return fmt.Errorf("moderate list %s: %w", target, err)
	}
	action := "unhide"
	if hidden {
		action = "hide"
	}
	slog.InfoContext(ctx, "admin action", "action", "lists."+action, "target", string(target), "id", id)
	return nil
}

// Resolve closes a report; the reporter may report the object again afterwards.
func (h *ModerationHandler) Resolve(ctx context.Context, reportID uuid.UUID) (ports.Report, error) {
	if h == nil || h.repo == nil {
		return ports.Report{}, contract.ErrInvalid
	}
	if reportID == uuid.Nil {
		return ports.Report{}, contract.ErrNotFound
	}
	r, err := h.repo.ResolveReport(ctx, reportID)
	if err != nil {
		return ports.Report{}, fmt.Errorf("resolve list report: %w", err)
	}
	slog.InfoContext(ctx, "admin action", "action", "lists.resolve_report", "target", "report", "id", reportID)
	return r, nil
}
