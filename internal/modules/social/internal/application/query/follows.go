package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/ports"
)

type ListFollowingHandler struct {
	repo ports.Reader
}

func NewListFollowingHandler(repo ports.Reader) *ListFollowingHandler {
	return &ListFollowingHandler{repo: repo}
}

func (h *ListFollowingHandler) Handle(ctx context.Context, followerID uuid.UUID, limit int) ([]ports.Follow, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	return h.repo.ListFollowing(ctx, followerID, limit)
}

type ListFollowersHandler struct {
	repo ports.Reader
}

func NewListFollowersHandler(repo ports.Reader) *ListFollowersHandler {
	return &ListFollowersHandler{repo: repo}
}

func (h *ListFollowersHandler) Handle(ctx context.Context, followeeID uuid.UUID, limit int) ([]ports.Follow, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	return h.repo.ListFollowers(ctx, followeeID, limit)
}
