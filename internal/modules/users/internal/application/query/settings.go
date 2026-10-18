package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

type GetSettingsHandler struct {
	repo ports.UserRepository
}

func NewGetSettingsHandler(repo ports.UserRepository) *GetSettingsHandler {
	return &GetSettingsHandler{repo: repo}
}

func (h *GetSettingsHandler) Handle(ctx context.Context, userID uuid.UUID) (domain.Settings, error) {
	if userID == uuid.Nil {
		return domain.Settings{}, contract.ErrNotFound
	}
	return h.repo.GetSettings(ctx, userID)
}
