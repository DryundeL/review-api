package routes

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/http/httperr"
	authmodule "review-api/internal/modules/auth"
	catalogmodule "review-api/internal/modules/catalog"
	feedmodule "review-api/internal/modules/feed"
	gamificationmodule "review-api/internal/modules/gamification"
	notificationsmodule "review-api/internal/modules/notifications"
	reviewsmodule "review-api/internal/modules/reviews"
	searchmodule "review-api/internal/modules/search"
	sitemapmodule "review-api/internal/modules/sitemap"
	socialmodule "review-api/internal/modules/social"
	usersmodule "review-api/internal/modules/users"
	"review-api/internal/platform/ratelimit"
)

type APIModules struct {
	Auth          *authmodule.Module
	Users         *usersmodule.Module
	Reviews       *reviewsmodule.Module
	Catalog       *catalogmodule.Module
	Social        *socialmodule.Module
	Feed          *feedmodule.Module
	Search        *searchmodule.Module
	Sitemap       *sitemapmodule.Module
	Notifications *notificationsmodule.Module
	Gamification  *gamificationmodule.Module
}

type APIOptions struct {
	Redis           *redis.Client
	RateLimitPerMin int
}

func SetupAPIRoutes(e *echo.Echo, modules APIModules, opts APIOptions) {
	v1 := e.Group("/v1")
	v1.Use(ratelimit.Middleware(opts.Redis, opts.RateLimitPerMin, time.Minute))
	profileAccess := echo.MiddlewareFunc(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(echo.Context) error { return httperr.New(404, "NOT_FOUND", "user not found") }
	})
	if modules.Users != nil {
		profileAccess = modules.Users.ProfileAccess()
	}
	if modules.Auth != nil {
		modules.Auth.RegisterHTTP(v1)
	}
	if modules.Users != nil {
		modules.Users.RegisterHTTP(v1)
	}
	if modules.Reviews != nil {
		modules.Reviews.RegisterHTTP(v1, profileAccess)
	}
	if modules.Catalog != nil {
		modules.Catalog.RegisterHTTP(v1)
	}
	if modules.Social != nil {
		modules.Social.RegisterHTTP(v1, profileAccess)
	}
	if modules.Feed != nil {
		modules.Feed.RegisterHTTP(v1)
	}
	if modules.Search != nil {
		modules.Search.RegisterHTTP(v1)
	}
	if modules.Sitemap != nil {
		modules.Sitemap.RegisterHTTP(v1)
	}
	if modules.Notifications != nil {
		modules.Notifications.RegisterHTTP(v1)
	}
	if modules.Gamification != nil {
		modules.Gamification.RegisterHTTP(v1, profileAccess)
	}
}
