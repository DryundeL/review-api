package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"review-api/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.Bootstrap(ctx)
	if err != nil {
		slog.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		slog.Error("application stopped with error", "error", err)
		os.Exit(1)
	}
	slog.Info("application shut down successfully")
}
