package app

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth"
	authcontract "review-api/internal/modules/auth/contract"
	"review-api/internal/modules/catalog"
	catalogcontract "review-api/internal/modules/catalog/contract"
	"review-api/internal/modules/feed"
	"review-api/internal/modules/gamification"
	"review-api/internal/modules/notifications"
	"review-api/internal/modules/reviews"
	reviewscontract "review-api/internal/modules/reviews/contract"
	"review-api/internal/modules/search"
	"review-api/internal/modules/sitemap"
	"review-api/internal/modules/social"
	"review-api/internal/modules/users"
	userscontract "review-api/internal/modules/users/contract"
	"review-api/internal/platform/config"
	"review-api/internal/platform/httpserver"
	mailx "review-api/internal/platform/mail"
)

type Modules struct {
	Auth          *auth.Module
	Users         *users.Module
	Reviews       *reviews.Module
	Catalog       *catalog.Module
	Social        *social.Module
	Feed          *feed.Module
	Search        *search.Module
	Sitemap       *sitemap.Module
	Notifications *notifications.Module
	Gamification  *gamification.Module
}

func WireModules(cfg *config.Config, pool *pgxpool.Pool, rdb *redis.Client) Modules {
	emailSender := mailx.New(mailx.Config{
		Host: cfg.App.SMTPHost, Port: cfg.App.SMTPPort, User: cfg.App.SMTPUser,
		Password: cfg.App.SMTPPassword, From: cfg.App.SMTPFrom,
	})
	usersMod := users.New(users.Dependencies{
		Pool: pool, Redis: rdb, AuthSecret: cfg.App.JWTSecret, EmailSender: emailSender,
	})
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
		EmailResolver: emailResolverAdapter{users: usersMod},
		EmailSender:   emailSender,
	})
	usersMod.SetAuth(authMod.Middlewares.Bearer)
	admin := httpserver.AdminToken(cfg.App.AdminToken)
	reviewsMod := reviews.New(reviews.Dependencies{
		Pool:                pool,
		Admin:               admin,
		Auth:                authMod.Middlewares.Bearer,
		Users:               usersMod,
		ReportHideThreshold: cfg.App.ReportHideThreshold,
	})
	catalogMod := catalog.New(catalog.Dependencies{
		Pool:  pool,
		Admin: admin,
		Types: reviewTypes{reviews: reviewsMod},
	})
	socialMod := social.New(social.Dependencies{
		Pool:  pool,
		Auth:  authMod.Middlewares.Bearer,
		Users: usersMod,
	})
	feedMod := feed.New(feed.Dependencies{
		Pool:  pool,
		Auth:  authMod.Middlewares.Bearer,
		Users: usersMod,
	})
	searchMod := search.New(search.Dependencies{
		Pool:  pool,
		Auth:  authMod.Middlewares.Bearer,
		Users: usersMod,
	})
	notificationsMod := notifications.New(notifications.Dependencies{
		Pool:    pool,
		Auth:    authMod.Middlewares.Bearer,
		Users:   usersMod,
		Reviews: reviewsMod,
	})
	gamificationMod := gamification.New(gamification.Dependencies{
		Pool:    pool,
		Redis:   rdb,
		Admin:   admin,
		Auth:    authMod.Middlewares.Bearer,
		Users:   usersMod,
		Reviews: reviewsMod,
		Points: gamification.PointsConfig{
			ReviewPerBlock: cfg.App.PointsReviewPerBlock,
			LikeGiven:      cfg.App.PointsLikeGiven,
			LikeReceived:   cfg.App.PointsLikeReceived,
			Follower:       cfg.App.PointsFollower,
		},
	})
	usersMod.SetCosmetics(gamificationMod)
	return Modules{
		Auth: authMod, Users: usersMod, Reviews: reviewsMod,
		Catalog: catalogMod, Social: socialMod, Feed: feedMod, Search: searchMod,
		Sitemap:       sitemap.New(sitemap.Dependencies{Pool: pool}),
		Notifications: notificationsMod, Gamification: gamificationMod,
	}
}

type emailResolverAdapter struct {
	users *users.Module
}

func (a emailResolverAdapter) ResolveLoginEmail(ctx context.Context, raw string) (string, int64, bool, error) {
	email, telegramID, found, err := a.users.ResolveLoginEmail(ctx, raw)
	var field userscontract.FieldError
	if errors.As(err, &field) {
		return "", 0, false, authcontract.ErrInvalidEmail
	}
	return email, telegramID, found, err
}

type reviewTypes struct {
	reviews *reviews.Module
}

func (r reviewTypes) RequireActive(ctx context.Context, codes []string) error {
	err := r.reviews.RequireActiveTypes(ctx, codes)
	var field reviewscontract.FieldError
	if errors.As(err, &field) {
		return catalogcontract.FieldError{Field: "type", Index: -1}
	}
	return err
}
