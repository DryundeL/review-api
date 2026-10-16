package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

type GetReviewHandler struct {
	repo ports.ReviewReader
}

func NewGetReviewHandler(repo ports.ReviewReader) *GetReviewHandler {
	return &GetReviewHandler{repo: repo}
}

// Handle returns a review. Drafts are visible only to the author (viewerID).
func (h *GetReviewHandler) Handle(ctx context.Context, id, viewerID uuid.UUID) (ports.Review, error) {
	if h == nil || h.repo == nil {
		return ports.Review{}, contract.ErrInvalid
	}
	item, err := h.repo.Get(ctx, id)
	if err != nil {
		return ports.Review{}, err
	}
	if item.Status == domain.StatusDraft && item.AuthorID != viewerID {
		return ports.Review{}, contract.ErrNotFound
	}
	return item, nil
}

type ListReviewsHandler struct {
	repo ports.ReviewReader
}

func NewListReviewsHandler(repo ports.ReviewReader) *ListReviewsHandler {
	return &ListReviewsHandler{repo: repo}
}

func (h *ListReviewsHandler) Handle(ctx context.Context, mediaObjectID uuid.UUID, limit int) ([]ports.Review, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	return h.repo.ListByMedia(ctx, mediaObjectID, limit)
}

func (h *ListReviewsHandler) ByAuthor(ctx context.Context, authorID uuid.UUID, publishedOnly bool, limit int) ([]ports.Review, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	return h.repo.ListByAuthor(ctx, authorID, publishedOnly, limit)
}
