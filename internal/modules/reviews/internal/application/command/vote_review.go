package command

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/reviews/internal/application/ports"
	"review-api/internal/modules/reviews/internal/domain"
	"review-api/internal/platform/transaction"
)

type VoteReviewHandler struct {
	repo      ports.ReviewWriter
	tx        transaction.Manager
	publisher ports.Publisher
}

func NewVoteReviewHandler(repo ports.ReviewWriter, tx transaction.Manager, publisher ports.Publisher) *VoteReviewHandler {
	return &VoteReviewHandler{repo: repo, tx: tx, publisher: publisher}
}

func (h *VoteReviewHandler) Handle(ctx context.Context, reviewID, userID uuid.UUID, isLike bool) error {
	if h == nil || h.repo == nil || h.tx == nil || h.publisher == nil {
		return contract.ErrInvalid
	}
	if reviewID == uuid.Nil || userID == uuid.Nil {
		return contract.ErrInvalid
	}
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		item, err := h.repo.Get(txCtx, reviewID)
		if err != nil {
			return err
		}
		if item.Status != domain.StatusPublished {
			return contract.ErrNotFound
		}
		if err := h.repo.UpsertVote(txCtx, reviewID, userID, isLike); err != nil {
			return err
		}
		return h.publisher.Publish(txCtx, contract.EventVoted, reviewID.String(), map[string]string{
			"reviewId": reviewID.String(),
			"userId":   userID.String(),
			"isLike":   strconv.FormatBool(isLike),
		})
	})
	if err != nil {
		return fmt.Errorf("vote review: %w", err)
	}
	return nil
}
