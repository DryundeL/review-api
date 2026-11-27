package quota

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"review-api/internal/platform/ratelimit"
)

// Redis limits uploads per user per window. Like the global /v1 limiter it fails open:
// a Redis outage must not break uploads; the global limiter and body size cap remain.
type Redis struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewRedis(rdb *redis.Client, limit int, window time.Duration) *Redis {
	return &Redis{rdb: rdb, limit: limit, window: window}
}

func (q *Redis) Allow(ctx context.Context, userID uuid.UUID) (bool, error) {
	if q == nil {
		return true, nil
	}
	ok, err := ratelimit.Allow(ctx, q.rdb, "rl:media-upload:"+userID.String(), q.limit, q.window)
	if err != nil {
		slog.WarnContext(ctx, "media upload quota unavailable", "error", err)
		return true, nil
	}
	return ok, nil
}
