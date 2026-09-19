package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"review-api/internal/http/routes"
	"review-api/internal/platform/config"
	"review-api/internal/platform/database"
	"review-api/internal/platform/httpserver"
	"review-api/internal/platform/observability"
	redisx "review-api/internal/platform/redis"
)

const serviceName = "review-api"

type App struct {
	Config      *config.Config
	DB          *pgxpool.Pool
	Redis       *redis.Client
	Router      *echo.Echo
	log         *slog.Logger
	stopTracing func(context.Context) error
}

func Bootstrap(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log := observability.Logger()

	stopTracing, err := observability.SetupTracing(ctx, serviceName, cfg.App.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("tracing: %w", err)
	}

	pool, err := database.Connect(ctx, cfg.Database.DSN())
	if err != nil {
		return nil, err
	}
	if err := database.MigrateUp(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	rdb, err := redisx.Connect(ctx, cfg.Redis)
	if err != nil {
		pool.Close()
		return nil, err
	}

	modules := WireModules(cfg, pool, rdb)

	router := httpserver.NewMux(httpserver.Options{
		Logger:      log,
		DB:          pool,
		Redis:       rdb,
		Service:     serviceName,
		Version:     cfg.App.Version,
		CORSOrigins: cfg.App.CORSOrigins,
		Env:         cfg.App.Env,
	})

	routes.SetupAPIRoutes(router, routes.APIModules{Auth: modules.Auth}, routes.APIOptions{
		Redis:           rdb,
		RateLimitPerMin: cfg.App.RateLimitPerMin,
	})

	return &App{
		Config:      cfg,
		DB:          pool,
		Redis:       rdb,
		Router:      router,
		log:         log,
		stopTracing: stopTracing,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         ":" + a.Config.App.Port,
		Handler:      a.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("server starting", "port", a.Config.App.Port, "env", a.Config.App.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		a.log.Info("application shutting down")
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http: %w", err)
		}
		return a.Close()
	case err := <-errCh:
		return err
	}
}

func (a *App) Close() error {
	if a.stopTracing != nil {
		_ = a.stopTracing(context.Background())
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
	return nil
}
