package query

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type ListAchievementsHandler struct {
	reader ports.AchievementReader
}

func NewListAchievementsHandler(reader ports.AchievementReader) *ListAchievementsHandler {
	return &ListAchievementsHandler{reader: reader}
}

func (h *ListAchievementsHandler) Handle(ctx context.Context, activeOnly bool) ([]ports.AchievementItem, error) {
	if h == nil || h.reader == nil {
		return nil, contract.ErrInvalid
	}
	items, err := h.reader.ListAchievements(ctx, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list achievements: %w", err)
	}
	return items, nil
}

type ListProgressHandler struct {
	reader ports.AchievementReader
}

func NewListProgressHandler(reader ports.AchievementReader) *ListProgressHandler {
	return &ListProgressHandler{reader: reader}
}

func (h *ListProgressHandler) Handle(ctx context.Context, userID uuid.UUID) ([]ports.AchievementProgress, error) {
	if h == nil || h.reader == nil || userID == uuid.Nil {
		return nil, contract.ErrInvalid
	}
	items, err := h.reader.ListProgress(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list achievement progress: %w", err)
	}
	return items, nil
}

func ProgressPercent(current, target int) int {
	if target < 1 {
		return 0
	}
	if current <= 0 {
		return 0
	}
	p := current * 100 / target
	if p > 100 {
		return 100
	}
	return p
}
