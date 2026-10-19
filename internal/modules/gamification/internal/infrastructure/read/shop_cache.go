package read

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/gamification/internal/application/ports"
)

const activeCatalogKey = "gamification:shop:active"

// ActiveCatalogTTL is how long the public shop list may stay stale after a write
// if invalidation is skipped (Redis blip). Writes still DEL the key.
const ActiveCatalogTTL = 5 * time.Minute

type ShopCatalogCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewShopCatalogCache(rdb *redis.Client) *ShopCatalogCache {
	return &ShopCatalogCache{rdb: rdb, ttl: ActiveCatalogTTL}
}

func (c *ShopCatalogCache) GetActive(ctx context.Context) ([]ports.ShopItem, bool, error) {
	if c == nil || c.rdb == nil {
		return nil, false, nil
	}
	raw, err := c.rdb.Get(ctx, activeCatalogKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("shop cache get: %w", err)
	}
	var items []ports.ShopItem
	if err := json.Unmarshal(raw, &items); err != nil {
		_ = c.Invalidate(ctx)
		return nil, false, nil
	}
	return items, true, nil
}

func (c *ShopCatalogCache) SetActive(ctx context.Context, items []ports.ShopItem) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if items == nil {
		items = []ports.ShopItem{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("shop cache marshal: %w", err)
	}
	ttl := c.ttl
	if ttl <= 0 {
		ttl = ActiveCatalogTTL
	}
	if err := c.rdb.Set(ctx, activeCatalogKey, raw, ttl).Err(); err != nil {
		return fmt.Errorf("shop cache set: %w", err)
	}
	return nil
}

func (c *ShopCatalogCache) Invalidate(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if err := c.rdb.Del(ctx, activeCatalogKey).Err(); err != nil {
		return fmt.Errorf("shop cache del: %w", err)
	}
	return nil
}

type activeCatalogStore interface {
	GetActive(ctx context.Context) ([]ports.ShopItem, bool, error)
	SetActive(ctx context.Context, items []ports.ShopItem) error
}

// CachedShopReader caches active-only lists; admin full lists always hit Postgres.
type CachedShopReader struct {
	inner ports.ShopReader
	cache activeCatalogStore
}

func NewCachedShopReader(inner ports.ShopReader, cache activeCatalogStore) *CachedShopReader {
	return &CachedShopReader{inner: inner, cache: cache}
}

func (r *CachedShopReader) ListItems(ctx context.Context, activeOnly bool) ([]ports.ShopItem, error) {
	if r == nil || r.inner == nil {
		return nil, fmt.Errorf("shop reader nil")
	}
	if !activeOnly || r.cache == nil {
		return r.inner.ListItems(ctx, activeOnly)
	}
	if items, ok, err := r.cache.GetActive(ctx); err == nil && ok {
		return items, nil
	}
	items, err := r.inner.ListItems(ctx, true)
	if err != nil {
		return nil, err
	}
	_ = r.cache.SetActive(ctx, items)
	return items, nil
}
