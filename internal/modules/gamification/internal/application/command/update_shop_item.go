package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type UpdateShopItemHandler struct {
	repo  ports.ShopWriter
	cache ports.CatalogInvalidator
}

func NewUpdateShopItemHandler(repo ports.ShopWriter, cache ports.CatalogInvalidator) *UpdateShopItemHandler {
	return &UpdateShopItemHandler{repo: repo, cache: cache}
}

func (h *UpdateShopItemHandler) Handle(ctx context.Context, id uuid.UUID, title *string, price *int, active *bool, sortOrder *int) (ports.ShopItem, error) {
	if h == nil || h.repo == nil || id == uuid.Nil {
		return ports.ShopItem{}, contract.ErrInvalid
	}
	if title != nil {
		t := strings.TrimSpace(*title)
		if len(t) < 1 || len(t) > 64 {
			return ports.ShopItem{}, contract.FieldError{Field: "title"}
		}
		title = &t
	}
	if price != nil && *price < 1 {
		return ports.ShopItem{}, contract.FieldError{Field: "price"}
	}
	if sortOrder != nil && (*sortOrder < 0 || *sortOrder > 10000) {
		return ports.ShopItem{}, contract.FieldError{Field: "sortOrder"}
	}
	item, err := h.repo.UpdateItem(ctx, id, title, price, active, sortOrder)
	if err != nil {
		return ports.ShopItem{}, fmt.Errorf("update shop item: %w", err)
	}
	if h.cache != nil {
		_ = h.cache.Invalidate(ctx)
	}
	return item, nil
}
