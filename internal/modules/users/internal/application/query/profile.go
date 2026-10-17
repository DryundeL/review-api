package query

import (
	"context"

	"github.com/google/uuid"

	"review-api/internal/modules/users/contract"
	"review-api/internal/modules/users/internal/application/ports"
	"review-api/internal/modules/users/internal/domain"
)

// Card is the profile payload for HTTP (own or public).
type Card struct {
	User           domain.User
	FollowersCount int64
	FollowingCount int64
	IsFollowing    bool
	IsMe           bool
}

type GetProfileHandler struct {
	repo ports.ProfileReader
}

func NewGetProfileHandler(repo ports.ProfileReader) *GetProfileHandler {
	return &GetProfileHandler{repo: repo}
}

func (h *GetProfileHandler) Handle(ctx context.Context, telegramID int64) (domain.User, error) {
	if h == nil || h.repo == nil || telegramID <= 0 {
		return domain.User{}, contract.ErrNotFound
	}
	return h.repo.FindByTelegramID(ctx, telegramID)
}

func (h *GetProfileHandler) Own(ctx context.Context, telegramID int64) (Card, error) {
	if h == nil || h.repo == nil || telegramID <= 0 {
		return Card{}, contract.ErrNotFound
	}
	card, err := h.repo.FindCardByTelegramID(ctx, telegramID, uuid.Nil)
	if err != nil {
		return Card{}, err
	}
	return Card{
		User:           card.User,
		FollowersCount: card.FollowersCount,
		FollowingCount: card.FollowingCount,
		IsFollowing:    false,
		IsMe:           true,
	}, nil
}

func (h *GetProfileHandler) ByID(ctx context.Context, userID, viewerID uuid.UUID) (Card, error) {
	if h == nil || h.repo == nil || userID == uuid.Nil {
		return Card{}, contract.ErrNotFound
	}
	card, err := h.repo.FindCardByID(ctx, userID, viewerID)
	if err != nil {
		return Card{}, err
	}
	isMe := viewerID != uuid.Nil && viewerID == card.User.ID
	isFollowing := card.IsFollowing && !isMe
	return Card{
		User:           card.User,
		FollowersCount: card.FollowersCount,
		FollowingCount: card.FollowingCount,
		IsFollowing:    isFollowing,
		IsMe:           isMe,
	}, nil
}
