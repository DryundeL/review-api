package routes

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	authmodule "review-api/internal/modules/auth"
	catalogmodule "review-api/internal/modules/catalog"
	feedmodule "review-api/internal/modules/feed"
	reviewsmodule "review-api/internal/modules/reviews"
	socialmodule "review-api/internal/modules/social"
	usersmodule "review-api/internal/modules/users"
	"review-api/internal/platform/ratelimit"
)

type APIModules struct {
	Auth    *authmodule.Module
	Users   *usersmodule.Module
	Reviews *reviewsmodule.Module
	Catalog *catalogmodule.Module
	Social  *socialmodule.Module
	Feed    *feedmodule.Module
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
	if modules.Users != nil {
		modules.Users.RegisterHTTP(v1)
	}
	if modules.Reviews != nil {
		modules.Reviews.RegisterHTTP(v1)
	}
	if modules.Catalog != nil {
		modules.Catalog.RegisterHTTP(v1)
	}
	if modules.Social != nil {
		modules.Social.RegisterHTTP(v1)
	}
	if modules.Feed != nil {
		modules.Feed.RegisterHTTP(v1)
	}
}
