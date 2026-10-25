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

func (h *FollowHandler) Handle(ctx context.Context, followerID, followeeID uuid.UUID) (string, error) {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return "", contract.ErrInvalid
	}
	item, err := domain.NewFollow(followerID, followeeID)
	if err != nil {
		return "", err
	}
	status := ""
	err = h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		status, err = h.repo.Create(txCtx, item)
		if err != nil {
			return err
		}
		if status == "pending" {
			return h.publisher.Publish(txCtx, contract.EventFollowRequested, followerID.String(), map[string]string{
				"followerId": followerID.String(), "followeeId": followeeID.String(), "notificationId": uuid.New().String(),
			})
		}
		return h.publisher.Publish(txCtx, contract.EventFollowed, followerID.String(), map[string]string{
			"followerId": followerID.String(),
			"followeeId": followeeID.String(),
		})
	})
	if err != nil {
		return "", fmt.Errorf("follow: %w", err)
	}
	return status, nil
}

type UnfollowHandler struct {
	tx   transaction.Manager
	repo ports.Writer
}

func NewUnfollowHandler(repo ports.Writer, tx transaction.Manager) *UnfollowHandler {
	return &UnfollowHandler{repo: repo, tx: tx}
}

func (h *UnfollowHandler) Handle(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil {
		return contract.ErrInvalid
	}
	if followerID == uuid.Nil || followeeID == uuid.Nil {
		return contract.ErrInvalid
	}
	if err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error { return h.repo.Delete(txCtx, followerID, followeeID) }); err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}
	return nil
}

// Approve creates a follow and publishes its event atomically, after owner approval.
func (h *FollowHandler) Approve(ctx context.Context, followerID, ownerID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return contract.ErrInvalid
	}
	item, err := domain.NewFollow(followerID, ownerID)
	if err != nil {
		return err
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := h.repo.Approve(txCtx, item); err != nil {
			return err
		}
		return h.publisher.Publish(txCtx, contract.EventFollowed, followerID.String(), map[string]string{"followerId": followerID.String(), "followeeId": ownerID.String()})
	})
}

func (h *FollowHandler) Reject(ctx context.Context, followerID, ownerID uuid.UUID) error {
	if h == nil || h.repo == nil || h.tx == nil {
		return contract.ErrInvalid
	}
	if followerID == uuid.Nil || ownerID == uuid.Nil {
		return contract.ErrInvalid
	}
	return h.tx.WithinTransaction(ctx, func(txCtx context.Context) error { return h.repo.DeleteRequest(txCtx, followerID, ownerID) })
}
