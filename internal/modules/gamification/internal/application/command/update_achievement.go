package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type UpdateAchievementHandler struct {
	repo  ports.AchievementWriter
	cache ports.CatalogInvalidator
}

func NewUpdateAchievementHandler(repo ports.AchievementWriter, cache ports.CatalogInvalidator) *UpdateAchievementHandler {
	return &UpdateAchievementHandler{repo: repo, cache: cache}
}

func (h *UpdateAchievementHandler) Handle(
	ctx context.Context, id uuid.UUID,
	title, description *string, target, rewardPoints, sortOrder *int, active *bool,
) (ports.AchievementItem, error) {
	if h == nil || h.repo == nil || id == uuid.Nil {
		return ports.AchievementItem{}, contract.ErrInvalid
	}
	if title != nil {
		t := strings.TrimSpace(*title)
		if len(t) < 1 || len(t) > 64 {
			return ports.AchievementItem{}, contract.FieldError{Field: "title"}
		}
		title = &t
	}
	if description != nil {
		d := strings.TrimSpace(*description)
		if len(d) < 1 || len(d) > 256 {
			return ports.AchievementItem{}, contract.FieldError{Field: "description"}
		}
		description = &d
	}
	if target != nil && *target < 1 {
		return ports.AchievementItem{}, contract.FieldError{Field: "target"}
	}
	if rewardPoints != nil && (*rewardPoints < 0 || *rewardPoints > 10000) {
		return ports.AchievementItem{}, contract.FieldError{Field: "rewardPoints"}
	}
	if sortOrder != nil && (*sortOrder < 0 || *sortOrder > 10000) {
		return ports.AchievementItem{}, contract.FieldError{Field: "sortOrder"}
	}
	item, err := h.repo.UpdateAchievement(ctx, id, title, description, target, rewardPoints, sortOrder, active)
	if err != nil {
		return ports.AchievementItem{}, fmt.Errorf("update achievement: %w", err)
	}
	if h.cache != nil {
		_ = h.cache.Invalidate(ctx)
	}
	return item, nil
}
