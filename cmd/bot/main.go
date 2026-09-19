package main

import (
	"log/slog"
	"os"
)

func main() {
	slog.Info("bot process is not in the platform+auth slice; notifications come later")
	os.Exit(0)
}
