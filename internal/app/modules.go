package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/google/uuid"

	"review-api/internal/modules/auth"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/catalog"
	catalogcontract "review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/reviews"
	reviewscontract "review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/config"
	"review-api/internal/platform/httpserver"
)

type Modules struct {
	Auth    *auth.Module
	Users   *users.Module
	Reviews *reviews.Module
	Catalog *catalog.Module
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
	reviewsMod := reviews.New(reviews.Dependencies{
		Pool:  pool,
		Admin: httpserver.AdminToken(cfg.App.AdminToken),
	})
	catalogMod := catalog.New(catalog.Dependencies{
		Pool:    pool,
		Auth:    authMod.Middlewares.Bearer,
		Types:   reviewTypes{reviews: reviewsMod},
		Authors: userAuthors{users: usersMod},
	})
	return Modules{Auth: authMod, Users: usersMod, Reviews: reviewsMod, Catalog: catalogMod}
}

type reviewTypes struct {
	reviews *reviews.Module
}

func (r reviewTypes) RequireActive(ctx context.Context, codes []string) error {
	err := r.reviews.RequireActiveTypes(ctx, codes)
	var field reviewscontract.FieldError
	if errors.As(err, &field) {
		return catalogcontract.FieldError{Field: "type"}
	}
	return err
}

type userAuthors struct {
	users *users.Module
}

func (a userAuthors) UserID(ctx context.Context, telegramID int64) (uuid.UUID, error) {
	id, err := a.users.UserID(ctx, telegramID)
	if errors.Is(err, userscontract.ErrNotFound) {
		return uuid.Nil, catalogcontract.ErrAuthorNotFound
	}
	return id, err
}
