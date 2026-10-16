package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

type ViewReviewHandler struct {
	repo      ports.ReviewWriter
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewViewReviewHandler(repo ports.ReviewWriter, tx transaction.Manager, publisher ports.Publisher) *ViewReviewHandler {
	return &ViewReviewHandler{repo: repo, tx: tx, publisher: publisher}
}

func (h *ViewReviewHandler) Handle(ctx context.Context, reviewID, userID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return contract.ErrInvalid
	}
	if reviewID == uuid.Nil || userID == uuid.Nil {
		return contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		item, err := h.repo.Get(txCtx, reviewID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusPublished {
			return contract.ErrNotFound
		}
		if item.AuthorID == userID {
			return nil
		}
		inserted, err := h.repo.InsertView(txCtx, reviewID, userID)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		// ponytail: outbox only on first view; batch feed later if volume hurts
		return h.publisher.Publish(txCtx, contract.EventViewed, reviewID.String(), map[string]string{
			"reviewId": reviewID.String(),
			"userId":   userID.String(),
		})
	})
	if err != nil {
		return fmt.Errorf("view review: %w", err)
	}
	return nil
}
