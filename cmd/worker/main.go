package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"review-api/internal/app/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := worker.Bootstrap(ctx)
	if err != nil {
		slog.Error("worker bootstrap failed", "error", err)
		os.Exit(1)
	}
	if err := application.Run(ctx); err != nil {
		slog.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
}
