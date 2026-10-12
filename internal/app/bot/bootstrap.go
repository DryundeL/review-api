package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"review-api/internal/modules/auth"
	"review-api/internal/modules/users"
	"review-api/internal/modules/users/contract"
	"review-api/internal/platform/config"
	"review-api/internal/platform/database"
	"review-api/internal/platform/observability"
	redisx "review-api/internal/platform/redis"
)

const serviceName = "review-api-bot"

type userEnsurer interface {
	EnsureUser(context.Context, contract.TelegramProfile) error
}

type App struct {
	db         *pgxpool.Pool
	redis      *redis.Client
	users      userEnsurer
	logins     loginGateway
	client     *Client
	log        *slog.Logger
	miniAppURL string
}

func Bootstrap(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if !validToken(cfg.App.TelegramBotToken) {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN format is invalid")
	}
	miniAppURL, err := parseMiniAppURL(cfg.App.MiniAppURL)
	if err != nil {
		return nil, err
	}
	pool, err := database.Connect(ctx, cfg.Database.DSN())
	if err != nil {
		return nil, err
	}
	rdb, err := redisx.Connect(ctx, cfg.Redis)
	if err != nil {
		pool.Close()
		return nil, err
	}
	authMod := auth.New(auth.Dependencies{
		BotToken:    cfg.App.TelegramBotToken,
		MaxAge:      cfg.App.AuthMaxAge,
		Redis:       rdb,
		JWTSecret:   cfg.App.JWTSecret,
		BotUsername: cfg.App.TelegramBotUsername,
	})
	return &App{
		db:         pool,
		redis:      rdb,
		users:      users.New(users.Dependencies{Pool: pool}),
		logins:     authMod,
		client:     NewClient(cfg.App.TelegramBotToken),
		log:        observability.Logger(),
		miniAppURL: miniAppURL,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	defer a.db.Close()
	if a.redis != nil {
		defer a.redis.Close()
	}
	a.log.Info("bot started", "service", serviceName)

	var offset int64
	for {
		if ctx.Err() != nil {
			a.log.Info("bot shutting down")
			return nil
		}
		started := time.Now()
		updates, err := a.client.GetUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				a.log.Info("bot shutting down")
				return nil
			}
			delay := retryDelay(err)
			a.log.Error("getUpdates failed", "error", err, "retry_after", delay.String())
			if !sleep(ctx, delay) {
				a.log.Info("bot shutting down")
				return nil
			}
			continue
		}
		if len(updates) == 0 && time.Since(started) < time.Second {
			if !sleep(ctx, time.Second) {
				a.log.Info("bot shutting down")
				return nil
			}
			continue
		}
		for _, update := range updates {
			if err := a.handle(ctx, update); err != nil {
				a.log.Error("handle update failed", "update_id", update.UpdateID, "error", err)
				if !sleep(ctx, retryDelay(err)) {
					a.log.Info("bot shutting down")
					return nil
				}
				break
			}
			offset = update.UpdateID + 1
		}
	}
}

func (a *App) handle(ctx context.Context, update Update) error {
	if update.CallbackQuery != nil {
		return a.handleLoginCallback(ctx, update.CallbackQuery)
	}
	if id, ok := loginStartID(update); ok {
		return a.handleLoginStart(ctx, update, id)
	}
	profile, chatID, ok := startProfile(update)
	if !ok {
		return nil
	}
	if err := a.users.EnsureUser(ctx, profile); err != nil {
		if errors.Is(err, contract.ErrInvalidProfile) {
			a.log.Warn("skip invalid start profile", "telegram_id", profile.TelegramID)
			return nil
		}
		return err
	}
	a.log.Info("user ensured from start", "telegram_id", profile.TelegramID)
	if chatID == 0 {
		return nil
	}
	view := startMessage(profile, a.miniAppURL)
	if err := a.client.SetChatMenuButton(ctx, chatID, view.MenuButton, a.miniAppURL); err != nil {
		var api *APIError
		if errors.As(err, &api) && !api.Temporary() {
			a.log.Warn("menu button not set", "telegram_id", profile.TelegramID, "error", err)
		} else {
			return err
		}
	}
	if err := a.client.SendMessage(ctx, chatID, view.Text, view.Markup); err != nil {
		var api *APIError
		if errors.As(err, &api) && !api.Temporary() {
			a.log.Warn("start reply not delivered", "telegram_id", profile.TelegramID, "error", err)
			return nil
		}
		return err
	}
	return nil
}

func retryDelay(err error) time.Duration {
	var api *APIError
	if errors.As(err, &api) {
		if api.RetryAfter > 0 {
			return api.RetryAfter
		}
		if api.Code == 401 {
			return 30 * time.Second
		}
	}
	return 2 * time.Second
}

func sleep(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
