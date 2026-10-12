package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth"
	"review-api/internal/modules/users"
	"review-api/internal/platform/config"
)

type Modules struct {
	Auth  *auth.Module
	Users *users.Module
}

func WireModules(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client) Modules {
	authMod := auth.New(auth.Dependencies{
		BotToken:    cfg.App.TelegramBotToken,
		MaxAge:      cfg.App.AuthMaxAge,
		Redis:       rdb,
		JWTSecret:   cfg.App.JWTSecret,
		BotUsername: cfg.App.TelegramBotUsername,
	})
	return Modules{
		Auth: authMod,
		Users: users.New(users.Dependencies{
			Pool: pool,
			Auth: authMod.Middlewares.Bearer,
		}),
	}
}
