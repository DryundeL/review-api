package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/platform/transaction"
)

type GrantPointsHandler struct {
	repo ports.WalletWriter
	tx   transaction.Manager
}

func NewGrantPointsHandler(repo ports.WalletWriter, tx transaction.Manager) *GrantPointsHandler {
	return &GrantPointsHandler{repo: repo, tx: tx}
}

func (h *GrantPointsHandler) Handle(ctx context.Context, userID uuid.UUID, amount int, reason, sourceKey string) error {
	if h == nil || h.repo == nil || h.tx == nil {
		return contract.ErrInvalid
	}
	if userID == uuid.Nil || amount < 1 || reason == "" || sourceKey == "" {
		return contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		_, err := h.repo.GrantPoints(txCtx, userID, amount, reason, sourceKey)
		return err
	})
	if err != nil {
		return fmt.Errorf("grant points: %w", err)
	}
	return nil
}
