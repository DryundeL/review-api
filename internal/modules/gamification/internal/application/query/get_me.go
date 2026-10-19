package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type GetMeHandler struct {
	repo ports.WalletReader
}

func NewGetMeHandler(repo ports.WalletReader) *GetMeHandler {
	return &GetMeHandler{repo: repo}
}

func (h *GetMeHandler) Handle(ctx context.Context, userID uuid.UUID) (ports.Me, error) {
	if h == nil || h.repo == nil || userID == uuid.Nil {
		return ports.Me{}, contract.ErrInvalid
	}
	return h.repo.GetMe(ctx, userID)
}
