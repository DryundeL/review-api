package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/notifications/contract"
	"review-api/internal/modules/notifications/internal/application/ports"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

type ListInboxHandler struct {
	repo ports.InboxRepository
}

func NewListInboxHandler(repo ports.InboxRepository) *ListInboxHandler {
	return &ListInboxHandler{repo: repo}
}

func (h *ListInboxHandler) Handle(ctx context.Context, userID uuid.UUID, limit, offset int) ([]ports.Notification, error) {
	if userID == uuid.Nil {
		return nil, contract.ErrNotFound
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return h.repo.ListInbox(ctx, userID, limit, offset)
}

type UnreadCountHandler struct {
	repo ports.InboxRepository
}

func NewUnreadCountHandler(repo ports.InboxRepository) *UnreadCountHandler {
	return &UnreadCountHandler{repo: repo}
}

func (h *UnreadCountHandler) Handle(ctx context.Context, userID uuid.UUID) (int64, error) {
	if userID == uuid.Nil {
		return 0, contract.ErrNotFound
	}
	return h.repo.UnreadCount(ctx, userID)
}
