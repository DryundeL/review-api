package query

import (
	"context"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
)

type ListShopItemsHandler struct {
	repo ports.ShopReader
}

func NewListShopItemsHandler(repo ports.ShopReader) *ListShopItemsHandler {
	return &ListShopItemsHandler{repo: repo}
}

func (h *ListShopItemsHandler) Handle(ctx context.Context, activeOnly bool) ([]ports.ShopItem, error) {
	if h == nil || h.repo == nil {
		return nil, contract.ErrInvalid
	}
	return h.repo.ListItems(ctx, activeOnly)
}
