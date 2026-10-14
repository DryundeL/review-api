package query

import (
	"context"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
)

type ListTypesHandler struct {
	repo ports.Reader
}

func NewListTypesHandler(repo ports.Reader) *ListTypesHandler {
	return &ListTypesHandler{repo: repo}
}

func (h *ListTypesHandler) Handle(ctx context.Context, activeOnly bool) ([]ports.Type, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrNotFound
	}
	return h.repo.List(ctx, activeOnly)
}
