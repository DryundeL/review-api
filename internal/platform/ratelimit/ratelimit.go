package ratelimit

import (
	"context"
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

			key := fmt.Sprintf("rl:%s", c.RealIP())
			n, err := hit(c.Request().Context(), rdb, key, window)
			if err != nil {
				return next(c)
			}
			if n > int64(limit) {
				return httperr.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			}
			return next(c)
		}
	}
}

// Allow засчитывает попытку по ключу и сообщает, укладывается ли она в limit за window.
// Ошибку Redis вызывающий код обрабатывает сам.
func Allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
	if rdb == nil || limit <= 0 {
		return true, nil
	}
	n, err := hit(ctx, rdb, key, window)
	if err != nil {
		return false, err
	}
	return n <= int64(limit), nil
}

// hit увеличивает счётчик окна. INCR и EXPIRE NX выполняются одной транзакцией
// и не отменяются вместе с запросом: иначе обрыв соединения между ними оставлял
// ключ без TTL, и IP блокировался навсегда. EXPIRE NX заодно восстанавливает
// TTL у уже застрявших ключей.
func hit(ctx context.Context, rdb *redis.Client, key string, window time.Duration) (int64, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	var incr *redis.IntCmd
	_, err := rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		incr = pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, window)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
