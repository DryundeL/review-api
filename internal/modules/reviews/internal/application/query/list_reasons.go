package query

import (
	"context"
	"fmt"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

type ListReasonsHandler struct {
	repo ports.ReasonReader
}

func NewListReasonsHandler(repo ports.ReasonReader) *ListReasonsHandler {
	return &ListReasonsHandler{repo: repo}
}

func (h *ListReasonsHandler) Handle(ctx context.Context) ([]ports.Reason, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	items, err := h.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list report reasons: %w", err)
	}
	return items, nil
}
