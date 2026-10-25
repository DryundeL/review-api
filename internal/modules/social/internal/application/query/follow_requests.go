package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/ports"
)

type FollowRequestsHandler struct{ repo ports.Reader }

func NewFollowRequestsHandler(repo ports.Reader) *FollowRequestsHandler {
	return &FollowRequestsHandler{repo: repo}
}
func (h *FollowRequestsHandler) List(ctx context.Context, owner uuid.UUID, limit, offset int) ([]ports.FollowUser, error) {
	if owner == uuid.Nil {
		return nil, contract.ErrInvalid
	}
	return h.repo.ListRequests(ctx, owner, limit, offset)
}
func (h *FollowRequestsHandler) Status(ctx context.Context, subject, viewer uuid.UUID) (string, error) {
	if subject == uuid.Nil {
		return "", contract.ErrNotFound
	}
	return h.repo.FollowStatus(ctx, subject, viewer)
}
