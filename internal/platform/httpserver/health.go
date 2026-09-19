package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type redisPinger struct {
	c *redis.Client
}

func (p redisPinger) Ping(ctx context.Context) error {
	return p.c.Ping(ctx).Err()
}

type HealthHandler struct {
	db      Pinger
	redis   Pinger
	service string
	version string
	started time.Time
}

func NewHealthHandler(db Pinger, redisClient *redis.Client, service, version string) *HealthHandler {
	if service == "" {
		service = "review-api"
	}
	if version == "" {
		version = "dev"
	}
	var rp Pinger
	if redisClient != nil {
		rp = redisPinger{c: redisClient}
	}
	return &HealthHandler{
		db:      db,
		redis:   rp,
		service: service,
		version: version,
		started: time.Now().UTC(),
	}
}

func RegisterHealthRoutes(e *echo.Echo, h *HealthHandler) {
	e.GET("/healthz", h.Live)
	e.GET("/readyz", h.Ready)
}

func (h *HealthHandler) Live(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":  "ok",
		"service": h.service,
		"version": h.version,
	})
}

func (h *HealthHandler) Ready(c echo.Context) error {
	ctx := c.Request().Context()
	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		}
	}
	if h.redis != nil {
		if err := h.redis.Ping(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":  "ready",
		"service": h.service,
		"version": h.version,
	})
}
