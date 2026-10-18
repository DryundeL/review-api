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

type CreateReviewHandler struct {
	repo      ports.ReviewWriter
	reader    ports.ReviewReader
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewCreateReviewHandler(
	repo ports.ReviewWriter,
	reader ports.ReviewReader,
	tx transaction.Manager,
	publisher ports.Publisher,
) *CreateReviewHandler {
	return &CreateReviewHandler{repo: repo, reader: reader, tx: tx, publisher: publisher}
}

func (h *CreateReviewHandler) Handle(ctx context.Context, authorID uuid.UUID, draft domain.Draft) (ports.Review, error) {
	if h == nil || h.repo == nil || h.reader == nil || h.tx == nil || h.publisher == nil {
		return ports.Review{}, contract.ErrInvalid
	}
	item, err := domain.NewReview(authorID, draft)
	if err != nil {
		return ports.Review{}, err
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Create(txCtx, item); err != nil {
			return err
		}
		if item.Status == domain.StatusPublished {
			return h.publisher.Publish(txCtx, contract.EventPublished, item.ID.String(), map[string]string{
				"id":            item.ID.String(),
				"authorId":      item.AuthorID.String(),
				"mediaObjectId": item.MediaObjectID.String(),
			})
		}
		return nil
	})
	if err != nil {
		return ports.Review{}, fmt.Errorf("create review: %w", err)
	}
	out, err := h.reader.Get(ctx, item.ID, authorID)
	if err != nil {
		return ports.Review{}, fmt.Errorf("create review read: %w", err)
	}
	return out, nil
}
