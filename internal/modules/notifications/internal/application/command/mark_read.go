package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/contract"
	"review-api/internal/modules/notifications/internal/application/ports"
)

type MarkReadHandler struct {
	repo ports.InboxRepository
}

func NewMarkReadHandler(repo ports.InboxRepository) *MarkReadHandler {
	return &MarkReadHandler{repo: repo}
}

func (h *MarkReadHandler) Handle(ctx context.Context, userID, id uuid.UUID) error {
	if userID == uuid.Nil || id == uuid.Nil {
		return contract.ErrNotFound
	}
	if err := h.repo.MarkRead(ctx, userID, id); err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

type MarkAllReadHandler struct {
	repo ports.InboxRepository
}

func NewMarkAllReadHandler(repo ports.InboxRepository) *MarkAllReadHandler {
	return &MarkAllReadHandler{repo: repo}
}

func (h *MarkAllReadHandler) Handle(ctx context.Context, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return contract.ErrNotFound
	}
	if err := h.repo.MarkAllRead(ctx, userID); err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}
