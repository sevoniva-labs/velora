package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sevoniva-labs/velora/server/internal/bootstrap"
)

var version = "0.2.0-dev"

func main() {
	if err := run(); err != nil {
		slog.Error("velora stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.New(ctx, bootstrap.Options{Version: version})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer app.Close()

	return app.Run(ctx)
}
