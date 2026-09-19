package routes

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	authmodule "review-api/internal/modules/auth"
	"review-api/internal/platform/ratelimit"
)

type APIModules struct {
	Auth *authmodule.Module
}

type APIOptions struct {
	Redis           *redis.Client
	RateLimitPerMin int
}

func SetupAPIRoutes(e *echo.Echo, modules APIModules, opts APIOptions) {
	v1 := e.Group("/v1")
	v1.Use(ratelimit.Middleware(opts.Redis, opts.RateLimitPerMin, time.Minute))
	if modules.Auth != nil {
		modules.Auth.RegisterHTTP(v1)
	}
}
