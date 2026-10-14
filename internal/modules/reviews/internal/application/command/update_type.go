package command

import (
	"context"
	"fmt"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
)

type UpdateTypeHandler struct {
	repo ports.Writer
}

func NewUpdateTypeHandler(repo ports.Writer) *UpdateTypeHandler {
	return &UpdateTypeHandler{repo: repo}
}

func (h *UpdateTypeHandler) Handle(ctx context.Context, code string, label *string, sortOrder *int, active *bool) (ports.Type, error) {
	if h == nil || h.repo == nil {
		return ports.Type{}, contract.ErrInvalid
	}
	if label == nil && sortOrder == nil && active == nil {
		return ports.Type{}, contract.FieldError{Field: "body"}
	}
	code, err := domain.ParseCode(code)
	if err != nil {
		return ports.Type{}, contract.FieldError{Field: "code"}
	}
	var normalized *string
	if label != nil {
		value, err := domain.ParseLabel(*label)
		if err != nil {
			return ports.Type{}, err
		}
		normalized = &value
	}
	if sortOrder != nil {
		if err := domain.ParseSort(*sortOrder); err != nil {
			return ports.Type{}, err
		}
	}
	item, err := h.repo.Update(ctx, code, normalized, sortOrder, active)
	if err != nil {
		return ports.Type{}, fmt.Errorf("update review type: %w", err)
	}
	return toType(item), nil
}
