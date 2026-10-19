package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type CosmeticsHandler struct {
	repo ports.WalletReader
}

func NewCosmeticsHandler(repo ports.WalletReader) *CosmeticsHandler {
	return &CosmeticsHandler{repo: repo}
}

func (h *CosmeticsHandler) Handle(ctx context.Context, userID uuid.UUID) (contract.Cosmetics, error) {
	if h == nil || h.repo == nil || userID == uuid.Nil {
		return contract.Cosmetics{}, nil
	}
	p, err := h.repo.GetCosmetics(ctx, userID)
	if err != nil {
		return contract.Cosmetics{}, err
	}
	return contract.Cosmetics{
		Background:  p.EquippedBackground,
		AvatarFrame: p.EquippedAvatarFrame,
		Backdrop:    p.EquippedBackdrop,
	}, nil
}
