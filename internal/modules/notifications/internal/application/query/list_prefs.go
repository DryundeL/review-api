package query

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/contract"
	"review-api/internal/modules/notifications/internal/application/ports"
)

type ListPrefsHandler struct {
	repo ports.PrefRepository
}

func NewListPrefsHandler(repo ports.PrefRepository) *ListPrefsHandler {
	return &ListPrefsHandler{repo: repo}
}

func (h *ListPrefsHandler) Handle(ctx context.Context, userID uuid.UUID) ([]ports.PrefItem, error) {
	if userID == uuid.Nil {
		return nil, contract.ErrNotFound
	}
	if err := h.repo.EnsureDefaults(ctx, userID); err != nil {
		return nil, fmt.Errorf("list prefs: %w", err)
	}
	return h.repo.List(ctx, userID)
}
