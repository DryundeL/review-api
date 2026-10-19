package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/platform/transaction"
)

type PurchaseHandler struct {
	repo ports.WalletWriter
	tx   transaction.Manager
}

func NewPurchaseHandler(repo ports.WalletWriter, tx transaction.Manager) *PurchaseHandler {
	return &PurchaseHandler{repo: repo, tx: tx}
}

func (h *PurchaseHandler) Handle(ctx context.Context, userID, itemID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil {
		return contract.ErrInvalid
	}
	if userID == uuid.Nil || itemID == uuid.Nil {
		return contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		return h.repo.Purchase(txCtx, userID, itemID)
	})
	if err != nil {
		return fmt.Errorf("purchase: %w", err)
	}
	return nil
}
