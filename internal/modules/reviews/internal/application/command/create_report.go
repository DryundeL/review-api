package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

type CreateReportHandler struct {
	repo      ports.ReviewWriter
	reasons   ports.ReasonReader
	tx        transaction.Manager
	publisher ports.Publisher
	threshold int
}

func NewCreateReportHandler(
	repo ports.ReviewWriter,
	reasons ports.ReasonReader,
	tx transaction.Manager,
	publisher ports.Publisher,
	threshold int,
) *CreateReportHandler {
	return &CreateReportHandler{
		repo:      repo,
		reasons:   reasons,
		tx:        tx,
		publisher: publisher,
		threshold: threshold,
	}
}

func (h *CreateReportHandler) Handle(
	ctx context.Context,
	reviewID, reporterID uuid.UUID,
	reasonCode string,
) (ports.Report, error) {
	if h == nil || h.repo == nil || h.reasons == nil || h.tx == nil || h.publisher == nil {
		return ports.Report{}, contract.ErrInvalid
	}
	if reviewID == uuid.Nil || reporterID == uuid.Nil {
		return ports.Report{}, contract.ErrInvalid
	}
	if h.threshold < 1 {
		return ports.Report{}, contract.ErrInvalid
	}
	reasonCode = strings.TrimSpace(reasonCode)
	if reasonCode == "" {
		return ports.Report{}, contract.FieldError{Field: "reasonCode"}
	}
	active, err := h.reasons.IsActive(ctx, reasonCode)
	if err != nil {
		return ports.Report{}, fmt.Errorf("create report: %w", err)
	}
	if !active {
		return ports.Report{}, contract.FieldError{Field: "reasonCode"}
	}

	report := ports.Report{
		ID:         uuid.New(),
		ReviewID:   reviewID,
		ReporterID: reporterID,
		ReasonCode: reasonCode,
		CreatedAt:  time.Now().UTC(),
	}

	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		item, err := h.repo.GetForUpdate(txCtx, reviewID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusPublished {
			return contract.ErrNotFound
		}
		if item.AuthorID == reporterID {
			return contract.ErrForbidden
		}
		if err := h.repo.InsertReport(txCtx, report); err != nil {
			return err
		}
		count, err := h.repo.CountDistinctReporters(txCtx, reviewID)
		if err != nil {
			return err
		}
		if count < h.threshold {
			return nil
		}
		if err := h.repo.Unpublish(txCtx, reviewID); err != nil {
			return err
		}
		return h.publisher.Publish(txCtx, contract.EventUnpublished, item.ID.String(), map[string]string{
			"id":            item.ID.String(),
			"authorId":      item.AuthorID.String(),
			"mediaObjectId": item.MediaObjectID.String(),
		})
	})
	if err != nil {
		return ports.Report{}, fmt.Errorf("create report: %w", err)
	}
	return report, nil
}
