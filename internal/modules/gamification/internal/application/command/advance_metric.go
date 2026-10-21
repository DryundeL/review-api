package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
	"review-api/internal/platform/transaction"
)

const ReasonAchievement = "achievement"

type AdvanceMetricHandler struct {
	achievements ports.AchievementWriter
	wallet       ports.WalletWriter
	publisher    ports.Publisher
	tx           transaction.Manager
}

func NewAdvanceMetricHandler(
	achievements ports.AchievementWriter,
	wallet ports.WalletWriter,
	publisher ports.Publisher,
	tx transaction.Manager,
) *AdvanceMetricHandler {
	return &AdvanceMetricHandler{
		achievements: achievements, wallet: wallet, publisher: publisher, tx: tx,
	}
}

// Handle advances a metric idempotently and unlocks crossed achievements in one TX.
// Returns (applied, newValue). newValue is 0 when not applied.
func (h *AdvanceMetricHandler) Handle(
	ctx context.Context, userID uuid.UUID, metric string, delta int, sourceKey string,
) (bool, int, error) {
	if h == nil || h.achievements == nil || h.wallet == nil || h.tx == nil {
		return false, 0, contract.ErrInvalid
	}
	if userID == uuid.Nil || delta < 1 || sourceKey == "" || !domain.ValidMetric(metric) {
		return false, 0, contract.ErrInvalid
	}
	var applied bool
	var newValue int
	err := h.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		ok, err := h.achievements.RecordMetricEvent(txCtx, userID, metric, delta, sourceKey)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		value, err := h.achievements.BumpMetric(txCtx, userID, metric, delta)
		if err != nil {
			return err
		}
		applied = true
		newValue = value
		candidates, err := h.achievements.ListUnlockable(txCtx, userID, metric, value)
		if err != nil {
			return err
		}
		for _, c := range candidates {
			inserted, err := h.achievements.UnlockAchievement(txCtx, userID, c.ID)
			if err != nil {
				return err
			}
			if !inserted {
				continue
			}
			if c.RewardPoints > 0 {
				if _, err := h.wallet.GrantPoints(
					txCtx, userID, c.RewardPoints, ReasonAchievement,
					"achievement:"+c.ID.String()+":"+userID.String(),
				); err != nil {
					return err
				}
			}
			if h.publisher != nil {
				if err := h.publisher.Publish(txCtx, contract.EventUnlockGranted, c.ID.String(), map[string]string{
					"userId":        userID.String(),
					"achievementId": c.ID.String(),
					"code":          c.Code,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return false, 0, fmt.Errorf("advance metric: %w", err)
	}
	return applied, newValue, nil
}
