package command

import (
	"context"
	"fmt"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

type CreateTypeHandler struct {
	repo ports.Writer
}

func NewCreateTypeHandler(repo ports.Writer) *CreateTypeHandler {
	return &CreateTypeHandler{repo: repo}
}

func (h *CreateTypeHandler) Handle(ctx context.Context, code, label string, sortOrder *int, active *bool) (ports.Type, error) {
	if h == nil || h.repo == nil {
		return ports.Type{}, contract.ErrInvalid
	}
	sort := 0
	if sortOrder == nil {
		next, err := h.repo.NextSort(ctx)
		if err != nil {
			return ports.Type{}, err
		}
		sort = next
	} else {
		sort = *sortOrder
	}
	isActive := true
	if active != nil {
		isActive = *active
	}
	item, err := domain.NewReviewType(code, label, sort, isActive)
	if err != nil {
		return ports.Type{}, err
	}
	if err := h.repo.Create(ctx, item); err != nil {
		return ports.Type{}, fmt.Errorf("create review type: %w", err)
	}
	return toType(item), nil
}

func toType(item domain.ReviewType) ports.Type {
	return ports.Type{Code: item.Code, Label: item.Label, SortOrder: item.SortOrder, IsActive: item.IsActive}
}
