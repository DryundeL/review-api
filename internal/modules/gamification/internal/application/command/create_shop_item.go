package command

import (
	"context"
	"fmt"
	"strings"

	"review-api/internal/modules/gamification/contract"
	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
)

type CreateShopItemHandler struct {
	repo  ports.ShopWriter
	cache ports.CatalogInvalidator
}

func NewCreateShopItemHandler(repo ports.ShopWriter, cache ports.CatalogInvalidator) *CreateShopItemHandler {
	return &CreateShopItemHandler{repo: repo, cache: cache}
}

func (h *CreateShopItemHandler) Handle(ctx context.Context, code, kind, title string, price int, sortOrder *int, active *bool) (ports.ShopItem, error) {
	if h == nil || h.repo == nil {
		return ports.ShopItem{}, contract.ErrInvalid
	}
	sort := 0
	if sortOrder == nil {
		next, err := h.repo.NextSort(ctx)
		if err != nil {
			return ports.ShopItem{}, err
		}
		sort = next
	} else {
		sort = *sortOrder
	}
	isActive := true
	if active != nil {
		isActive = *active
	}
	item, err := domain.NewShopItem(code, kind, title, price, sort, isActive)
	if err != nil {
		return ports.ShopItem{}, err
	}
	if err := h.repo.CreateItem(ctx, item); err != nil {
		return ports.ShopItem{}, fmt.Errorf("create shop item: %w", err)
	}
	if h.cache != nil {
		_ = h.cache.Invalidate(ctx)
	}
	return ports.ShopItem{
		ID: item.ID, Code: item.Code, Kind: item.Kind, Title: item.Title,
		Price: item.Price, IsActive: item.IsActive, SortOrder: item.SortOrder,
	}, nil
}

func NormalizeKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "background":
		return domain.KindBackground
	case "avatarFrame", "avatar_frame":
		return domain.KindAvatarFrame
	case "backdrop":
		return domain.KindBackdrop
	default:
		return strings.TrimSpace(kind)
	}
}
