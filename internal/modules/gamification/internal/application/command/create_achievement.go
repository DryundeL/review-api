package command

import (
	"context"
	"fmt"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
)

type CreateAchievementHandler struct {
	repo  ports.AchievementWriter
	cache ports.CatalogInvalidator
}

func NewCreateAchievementHandler(repo ports.AchievementWriter, cache ports.CatalogInvalidator) *CreateAchievementHandler {
	return &CreateAchievementHandler{repo: repo, cache: cache}
}

func (h *CreateAchievementHandler) Handle(
	ctx context.Context,
	code, title, description, metric string,
	target, rewardPoints int,
	sortOrder *int,
	active *bool,
) (ports.AchievementItem, error) {
	if h == nil || h.repo == nil {
		return ports.AchievementItem{}, contract.ErrInvalid
	}
	sort := 0
	if sortOrder == nil {
		next, err := h.repo.NextAchievementSort(ctx)
		if err != nil {
			return ports.AchievementItem{}, err
		}
		sort = next
	} else {
		sort = *sortOrder
	}
	isActive := true
	if active != nil {
		isActive = *active
	}
	item, err := domain.NewAchievement(code, title, description, metric, target, rewardPoints, sort, isActive)
	if err != nil {
		return ports.AchievementItem{}, err
	}
	if err := h.repo.CreateAchievement(ctx, item); err != nil {
		return ports.AchievementItem{}, fmt.Errorf("create achievement: %w", err)
	}
	if h.cache != nil {
		_ = h.cache.Invalidate(ctx)
	}
	return ports.AchievementItem{
		ID: item.ID, Code: item.Code, Title: item.Title, Description: item.Description,
		Metric: item.Metric, Target: item.Target, RewardPoints: item.RewardPoints,
		IsActive: item.IsActive, SortOrder: item.SortOrder,
	}, nil
}
