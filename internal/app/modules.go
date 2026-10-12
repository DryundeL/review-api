package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/config"
)

type Modules struct {
	Auth  *auth.Module
	Users *users.Module
}

func WireModules(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client) Modules {
	usersMod := users.New(users.Dependencies{Pool: pool})
	authMod := auth.New(auth.Dependencies{
		BotToken:    cfg.App.TelegramBotToken,
		MaxAge:      cfg.App.AuthMaxAge,
		Redis:       rdb,
		JWTSecret:   cfg.App.JWTSecret,
		BotUsername: cfg.App.TelegramBotUsername,
		EnsureUser: func(ctx context.Context, id authcontract.TelegramIdentity) error {
			err := usersMod.EnsureIdentity(ctx, id)
			if errors.Is(err, userscontract.ErrInvalidProfile) {
				return authcontract.ErrUnauthorized
			}
			return err
		},
	})
	usersMod.SetAuth(authMod.Middlewares.Bearer)
	return Modules{Auth: authMod, Users: usersMod}
}
