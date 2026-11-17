package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

type CreateReviewHandler struct {
	repo   ports.ReviewWriter
	reader ports.ReviewReader
}

func NewCreateReviewHandler(repo ports.ReviewWriter, reader ports.ReviewReader) *CreateReviewHandler {
	return &CreateReviewHandler{repo: repo, reader: reader}
}

// Handle creates a draft on the first edit. If the author already has a review of this
// media object (another tab, retry), it returns that review untouched and created=false.
func (h *CreateReviewHandler) Handle(ctx context.Context, authorID, mediaObjectID uuid.UUID, draft domain.Draft) (ports.EditorReview, bool, error) {
	if h == nil || h.repo == nil || h.reader == nil {
		return ports.EditorReview{}, false, contract.ErrInvalid
	}
	item, err := domain.NewReview(authorID, mediaObjectID, draft)
	if err != nil {
		return ports.EditorReview{}, false, err
	}
	created, err := h.repo.Create(ctx, item)
	if err != nil {
		return ports.EditorReview{}, false, fmt.Errorf("create review: %w", err)
	}
	out, err := h.reader.EditorByMedia(ctx, authorID, mediaObjectID)
	if err != nil {
		return ports.EditorReview{}, false, fmt.Errorf("create review read: %w", err)
	}
	return out, created, nil
}
