package read

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"review-api/internal/modules/gamification/internal/application/ports"
)

type memInner struct {
	calls int
	items []ports.ShopItem
}

func (m *memInner) ListItems(context.Context, bool) ([]ports.ShopItem, error) {
	m.calls++
	return m.items, nil
}

type memCache struct {
	items []ports.ShopItem
	hit   bool
}

func (c *memCache) GetActive(context.Context) ([]ports.ShopItem, bool, error) {
	if !c.hit {
		return nil, false, nil
	}
	return c.items, true, nil
}

func (c *memCache) SetActive(_ context.Context, items []ports.ShopItem) error {
	c.items = items
	c.hit = true
	return nil
}

func TestCachedShopReader_ActiveHitMiss(t *testing.T) {
	t.Parallel()
	item := ports.ShopItem{ID: uuid.New(), Code: "frame_gold", Kind: "avatar_frame", Price: 10, IsActive: true}
	inner := &memInner{items: []ports.ShopItem{item}}
	cache := &memCache{}
	r := NewCachedShopReader(inner, cache)

	got, err := r.ListItems(context.Background(), true)
	if err != nil || len(got) != 1 || inner.calls != 1 {
		t.Fatalf("miss: got=%v err=%v calls=%d", got, err, inner.calls)
	}
	got, err = r.ListItems(context.Background(), true)
	if err != nil || len(got) != 1 || inner.calls != 1 {
		t.Fatalf("hit: got=%v err=%v calls=%d", got, err, inner.calls)
	}
	_, _ = r.ListItems(context.Background(), false)
	if inner.calls != 2 {
		t.Fatalf("admin list must bypass cache, calls=%d", inner.calls)
	}
}
