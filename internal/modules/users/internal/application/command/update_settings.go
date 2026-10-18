package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

type UpdateSettingsHandler struct {
	repo ports.UserRepository
}

func NewUpdateSettingsHandler(repo ports.UserRepository) *UpdateSettingsHandler {
	return &UpdateSettingsHandler{repo: repo}
}

type UpdateSettingsInput struct {
	Language           *string
	AllowNotifications *bool
}

func (h *UpdateSettingsHandler) Handle(ctx context.Context, userID uuid.UUID, in UpdateSettingsInput) (domain.Settings, error) {
	if userID == uuid.Nil {
		return domain.Settings{}, contract.ErrNotFound
	}
	if in.Language == nil && in.AllowNotifications == nil {
		s, err := h.repo.GetSettings(ctx, userID)
		if err != nil {
			return domain.Settings{}, err
		}
		return s, nil
	}
	var language *string
	if in.Language != nil {
		cleaned, err := domain.SettingsLanguage(*in.Language)
		if err != nil {
			return domain.Settings{}, err
		}
		language = &cleaned
	}
	s, err := h.repo.UpdateSettings(ctx, userID, language, in.AllowNotifications)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("update settings: %w", err)
	}
	return s, nil
}
