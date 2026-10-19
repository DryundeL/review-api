package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
	"review-api/internal/platform/transaction"
)

type EquipInput struct {
	Background  *string
	AvatarFrame *string
	Backdrop    *string
}

type EquipHandler struct {
	repo ports.WalletWriter
	tx   transaction.Manager
}

func NewEquipHandler(repo ports.WalletWriter, tx transaction.Manager) *EquipHandler {
	return &EquipHandler{repo: repo, tx: tx}
}

func (h *EquipHandler) Handle(ctx context.Context, userID uuid.UUID, in EquipInput) (ports.Profile, error) {
	if h == nil || h.repo == nil || h.tx == nil {
		return ports.Profile{}, contract.ErrInvalid
	}
	if userID == uuid.Nil {
		return ports.Profile{}, contract.ErrInvalid
	}
	bg, err := normalizeEquip(in.Background)
	if err != nil {
		return ports.Profile{}, err
	}
	af, err := normalizeEquip(in.AvatarFrame)
	if err != nil {
		return ports.Profile{}, err
	}
	bd, err := normalizeEquip(in.Backdrop)
	if err != nil {
		return ports.Profile{}, err
	}
	var out ports.Profile
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		var e error
		out, e = h.repo.Equip(txCtx, userID, bg, af, bd)
		return e
	})
	if err != nil {
		return ports.Profile{}, fmt.Errorf("equip: %w", err)
	}
	return out, nil
}

func normalizeEquip(v *string) (*string, error) {
	if v == nil {
		return nil, nil
	}
	s := strings.TrimSpace(strings.ToLower(*v))
	if s == "" {
		empty := ""
		return &empty, nil
	}
	if !domain.ValidCode(s) {
		return nil, contract.FieldError{Field: "code"}
	}
	return &s, nil
}
