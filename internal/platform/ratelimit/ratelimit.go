package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/http/httperr"
)

func Middleware(rdb *redis.Client, limit int, window time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if rdb == nil || limit <= 0 {
				return next(c)
			}

			ip := c.RealIP()
			key := fmt.Sprintf("rl:%s", ip)
			ctx := c.Request().Context()

			n, err := rdb.Incr(ctx, key).Result()
			if err != nil {
				return next(c)
			}
			if n == 1 {
				_ = rdb.Expire(ctx, key, window).Err()
			}
			if n > int64(limit) {
				return httperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			}
			return next(c)
		}
	}
}
