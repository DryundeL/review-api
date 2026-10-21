package read

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/gamification/internal/application/ports"
)

const activeAchievementsKey = "gamification:achievements:active"

const ActiveAchievementsTTL = 5 * time.Minute

type AchievementsCatalogCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewAchievementsCatalogCache(rdb *redis.Client) *AchievementsCatalogCache {
	return &AchievementsCatalogCache{rdb: rdb, ttl: ActiveAchievementsTTL}
}

func (c *AchievementsCatalogCache) GetActive(ctx context.Context) ([]ports.AchievementItem, bool, error) {
	if c == nil || c.rdb == nil {
		return nil, false, nil
	}
	raw, err := c.rdb.Get(ctx, activeAchievementsKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("achievements cache get: %w", err)
	}
	var items []ports.AchievementItem
	if err := json.Unmarshal(raw, &items); err != nil {
		_ = c.Invalidate(ctx)
		return nil, false, nil
	}
	return items, true, nil
}

func (c *AchievementsCatalogCache) SetActive(ctx context.Context, items []ports.AchievementItem) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if items == nil {
		items = []ports.AchievementItem{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("achievements cache marshal: %w", err)
	}
	ttl := c.ttl
	if ttl <= 0 {
		ttl = ActiveAchievementsTTL
	}
	if err := c.rdb.Set(ctx, activeAchievementsKey, raw, ttl).Err(); err != nil {
		return fmt.Errorf("achievements cache set: %w", err)
	}
	return nil
}

func (c *AchievementsCatalogCache) Invalidate(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	if err := c.rdb.Del(ctx, activeAchievementsKey).Err(); err != nil {
		return fmt.Errorf("achievements cache del: %w", err)
	}
	return nil
}

type activeAchievementsStore interface {
	GetActive(ctx context.Context) ([]ports.AchievementItem, bool, error)
	SetActive(ctx context.Context, items []ports.AchievementItem) error
}

type CachedAchievementReader struct {
	inner ports.AchievementReader
	cache activeAchievementsStore
}

func NewCachedAchievementReader(inner ports.AchievementReader, cache activeAchievementsStore) *CachedAchievementReader {
	return &CachedAchievementReader{inner: inner, cache: cache}
}

func (r *CachedAchievementReader) ListAchievements(ctx context.Context, activeOnly bool) ([]ports.AchievementItem, error) {
	if r == nil || r.inner == nil {
		return nil, fmt.Errorf("achievement reader nil")
	}
	if !activeOnly || r.cache == nil {
		return r.inner.ListAchievements(ctx, activeOnly)
	}
	if items, ok, err := r.cache.GetActive(ctx); err == nil && ok {
		return items, nil
	}
	items, err := r.inner.ListAchievements(ctx, true)
	if err != nil {
		return nil, err
	}
	_ = r.cache.SetActive(ctx, items)
	return items, nil
}

func (r *CachedAchievementReader) ListProgress(ctx context.Context, userID uuid.UUID) ([]ports.AchievementProgress, error) {
	if r == nil || r.inner == nil {
		return nil, fmt.Errorf("achievement reader nil")
	}
	return r.inner.ListProgress(ctx, userID)
}
