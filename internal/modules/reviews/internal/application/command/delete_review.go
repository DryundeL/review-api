package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

type DeleteReviewHandler struct {
	repo ports.ReviewWriter
}

func NewDeleteReviewHandler(repo ports.ReviewWriter) *DeleteReviewHandler {
	return &DeleteReviewHandler{repo: repo}
}

func (h *DeleteReviewHandler) Handle(ctx context.Context, reviewID, authorID uuid.UUID) error {
	if h == nil || h.repo == nil {
		return contract.ErrInvalid
	}
	if reviewID == uuid.Nil || authorID == uuid.Nil {
		return contract.ErrInvalid
	}
	item, err := h.repo.Get(ctx, reviewID)
	if err != nil {
		return err
	}
	if item.AuthorID != authorID {
		return contract.ErrForbidden
	}
	if err := h.repo.Delete(ctx, reviewID); err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	return nil
}
