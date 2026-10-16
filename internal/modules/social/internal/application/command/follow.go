package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/social/contract"
	"review-api/internal/modules/social/internal/application/ports"
	"review-api/internal/modules/social/internal/domain"
	"review-api/internal/platform/transaction"
)

type FollowHandler struct {
	repo      ports.Writer
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewFollowHandler(repo ports.Writer, tx transaction.Manager, publisher ports.Publisher) *FollowHandler {
	return &FollowHandler{repo: repo, tx: tx, publisher: publisher}
}

func (h *FollowHandler) Handle(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return contract.ErrInvalid
	}
	item, err := domain.NewFollow(followerID, followeeID)
	if err != nil {
		return err
	}
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Create(txCtx, item); err != nil {
			return err
		}
		return h.publisher.Publish(txCtx, contract.EventFollowed, followerID.String(), map[string]string{
			"followerId": followerID.String(),
			"followeeId": followeeID.String(),
		})
	})
	if err != nil {
		return fmt.Errorf("follow: %w", err)
	}
	return nil
}

type UnfollowHandler struct {
	repo ports.Writer
}

func NewUnfollowHandler(repo ports.Writer) *UnfollowHandler {
	return &UnfollowHandler{repo: repo}
}

func (h *UnfollowHandler) Handle(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if h == nil || h.repo == nil {
		return contract.ErrInvalid
	}
	if followerID == uuid.Nil || followeeID == uuid.Nil {
		return contract.ErrInvalid
	}
	if err := h.repo.Delete(ctx, followerID, followeeID); err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}
	return nil
}
