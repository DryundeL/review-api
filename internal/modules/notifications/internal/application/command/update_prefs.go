package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/contract"
	"review-api/internal/modules/notifications/internal/application/ports"
)

type UpdatePrefsHandler struct {
	repo ports.PrefRepository
}

func NewUpdatePrefsHandler(repo ports.PrefRepository) *UpdatePrefsHandler {
	return &UpdatePrefsHandler{repo: repo}
}

func (h *UpdatePrefsHandler) Handle(ctx context.Context, userID uuid.UUID, items []ports.PrefPatch) ([]ports.PrefItem, error) {
	if userID == uuid.Nil {
		return nil, contract.ErrNotFound
	}
	if len(items) == 0 {
		return nil, contract.FieldError{Field: "items"}
	}
	var like, follow *bool
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Name == "" {
			return nil, contract.FieldError{Field: "name"}
		}
		if _, dup := seen[item.Name]; dup {
			return nil, contract.FieldError{Field: "name"}
		}
		seen[item.Name] = struct{}{}
		enabled := item.Enabled
		switch item.Name {
		case "like":
			like = &enabled
		case "follow":
			follow = &enabled
		default:
			return nil, contract.FieldError{Field: "name"}
		}
	}
	if err := h.repo.EnsureDefaults(ctx, userID); err != nil {
		return nil, fmt.Errorf("update prefs: %w", err)
	}
	if err := h.repo.Patch(ctx, userID, like, follow); err != nil {
		return nil, fmt.Errorf("update prefs: %w", err)
	}
	return h.repo.List(ctx, userID)
}
