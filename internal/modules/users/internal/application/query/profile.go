package query

import (
	"context"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

type GetProfileHandler struct {
	repo ports.ProfileReader
}

func NewGetProfileHandler(repo ports.ProfileReader) *GetProfileHandler {
	return &GetProfileHandler{repo: repo}
}

func (h *GetProfileHandler) Handle(ctx context.Context, telegramID int64) (domain.User, error) {
	if h == nil || h.repo == nil || telegramID <= 0 {
		return domain.User{}, contract.ErrNotFound
	}
	return h.repo.FindByTelegramID(ctx, telegramID)
}
