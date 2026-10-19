package command

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/application/ports"
	"review-api/internal/modules/gamification/internal/domain"
)

type memShopWriter struct {
	items []ports.ShopItem
}

func (m *memShopWriter) CreateItem(_ context.Context, item domain.ShopItem) error {
	m.items = append(m.items, ports.ShopItem{
		ID: item.ID, Code: item.Code, Kind: item.Kind, Title: item.Title,
		Price: item.Price, IsActive: item.IsActive, SortOrder: item.SortOrder,
	})
	return nil
}

func (m *memShopWriter) UpdateItem(_ context.Context, id uuid.UUID, title *string, price *int, active *bool, sortOrder *int) (ports.ShopItem, error) {
	return ports.ShopItem{ID: id, Code: "frame_gold", Kind: domain.KindAvatarFrame, Title: "x", Price: 1, IsActive: true}, nil
}

func (m *memShopWriter) GetItem(context.Context, uuid.UUID) (ports.ShopItem, error) {
	return ports.ShopItem{}, nil
}

func (m *memShopWriter) NextSort(context.Context) (int, error) { return 10, nil }

type memInvalidator struct{ n int }

func (m *memInvalidator) Invalidate(context.Context) error {
	m.n++
	return nil
}

func TestCreateInvalidatesCatalogCache(t *testing.T) {
	t.Parallel()
	inv := &memInvalidator{}
	h := NewCreateShopItemHandler(&memShopWriter{}, inv)
	if _, err := h.Handle(context.Background(), "frame_gold", domain.KindAvatarFrame, "Gold", 50, nil, nil); err != nil {
		t.Fatal(err)
	}
	if inv.n != 1 {
		t.Fatalf("invalidate calls=%d", inv.n)
	}
}
