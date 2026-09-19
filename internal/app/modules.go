package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth"
	"review-api/internal/platform/config"
)

type Modules struct {
	Auth *auth.Module
}

func WireModules(cfg *config.Config, _ *pgxpool.Pool, _ *redis.Client) Modules {
	return Modules{
		Auth: auth.New(auth.Dependencies{
			BotToken: cfg.App.TelegramBotToken,
			MaxAge:   cfg.App.AuthMaxAge,
		}),
	}
}
