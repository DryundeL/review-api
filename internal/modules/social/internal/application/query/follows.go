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

func (h *ListFollowingHandler) Handle(ctx context.Context, subjectID, viewerID uuid.UUID, limit, offset int) ([]ports.FollowUser, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	if err := ensureSubject(ctx, h.repo, subjectID); err != nil {
		return nil, err
	}
	return h.repo.ListFollowing(ctx, subjectID, viewerID, limit, offset)
}

type ListFollowersHandler struct {
	repo ports.Reader
}

func NewListFollowersHandler(repo ports.Reader) *ListFollowersHandler {
	return &ListFollowersHandler{repo: repo}
}

func (h *ListFollowersHandler) Handle(ctx context.Context, subjectID, viewerID uuid.UUID, limit, offset int) ([]ports.FollowUser, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	if err := ensureSubject(ctx, h.repo, subjectID); err != nil {
		return nil, err
	}
	return h.repo.ListFollowers(ctx, subjectID, viewerID, limit, offset)
}

func ensureSubject(ctx context.Context, repo ports.Reader, subjectID uuid.UUID) error {
	if subjectID == uuid.Nil {
		return contract.FieldError{Field: "id"}
	}
	ok, err := repo.UserExists(ctx, subjectID)
	if err != nil {
		return err
	}
	if !ok {
		return contract.ErrNotFound
	}
	return nil
}
