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

type UpdateReviewHandler struct {
	repo      ports.ReviewWriter
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewUpdateReviewHandler(repo ports.ReviewWriter, tx transaction.Manager, publisher ports.Publisher) *UpdateReviewHandler {
	return &UpdateReviewHandler{repo: repo, tx: tx, publisher: publisher}
}

func (h *UpdateReviewHandler) Handle(ctx context.Context, reviewID, authorID uuid.UUID, draft domain.Draft) (ports.Review, error) {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return ports.Review{}, contract.ErrInvalid
	}
	var out ports.Review
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		item, err := h.repo.Get(txCtx, reviewID)
		if err != nil {
			return err
		}
		if item.AuthorID != authorID {
			return contract.ErrForbidden
		}
		wasPublished := item.Status == domain.StatusPublished
		if err := item.Apply(draft); err != nil {
			return err
		}
		if err := h.repo.Replace(txCtx, item); err != nil {
			return err
		}
		if !wasPublished && item.Status == domain.StatusPublished {
			if err := h.publisher.Publish(txCtx, contract.EventPublished, item.ID.String(), map[string]string{
				"id":            item.ID.String(),
				"authorId":      item.AuthorID.String(),
				"mediaObjectId": item.MediaObjectID.String(),
			}); err != nil {
				return err
			}
		}
		out = toPortReview(item)
		return nil
	})
	if err != nil {
		return ports.Review{}, fmt.Errorf("update review: %w", err)
	}
	return out, nil
}
