package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"review-api/internal/platform/config"
	"review-api/internal/platform/database"
	"review-api/internal/platform/eventbus"
	"review-api/internal/platform/observability"
	"review-api/internal/platform/outbox"
)

const serviceName = "review-api-worker"

type App struct {
	DB       *pgxpool.Pool
	Relayer  outbox.Relayer
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func Bootstrap(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	log := observability.Logger()

	pool, err := database.Connect(ctx, cfg.Database.DSN())
	if err != nil {
		return nil, err
	}

	dispatcher := eventbus.NewDispatcher()
	bus := eventbus.Fanout{Pubs: []eventbus.Publisher{
		eventbus.LoggingPublisher{Log: log},
		dispatcher,
	}}
	relayer := outbox.NewRelayer(pool, bus, outbox.RelayerOptions{Logger: log})

	return &App{
		DB:       pool,
		Relayer:  relayer,
		log:      log,
		interval: cfg.App.WorkerPoll,
		batch:    cfg.App.WorkerBatch,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.log.Info("worker started", "service", serviceName, "poll_interval", a.interval.String(), "batch_size", a.batch)
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	if err := a.relayOnce(ctx); err != nil {
		a.log.Error("relay tick failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			a.log.Info("worker shutting down")
			a.DB.Close()
			return nil
		case <-ticker.C:
			if err := a.relayOnce(ctx); err != nil {
				a.log.Error("relay tick failed", "error", err)
			}
		}
	}
}

func (a *App) relayOnce(ctx context.Context) error {
	n, err := a.Relayer.RelayPending(ctx, a.batch)
	if err != nil {
		return err
	}
	if n > 0 {
		a.log.Info("relay batch done", "published", n)
	}
	return nil
}
