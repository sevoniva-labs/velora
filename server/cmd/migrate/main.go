// velora-migrate is the one-shot database schema migration command intended for
// release pipelines. Production Kubernetes deployments should run it once
// before rolling out API/Worker replicas instead of enabling per-Pod migration.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/logx"
)

var version = "0.2.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.LoadForMigration()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(2)
	}
	log := logx.New(cfg.Observability.LogLevel, cfg.Observability.LogFormat, cfg.App.Name+"-migrate", cfg.App.Environment, version)
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	if err := database.Migrate(db.DB, cfg.Database.Provider); err != nil {
		log.Error("migration", "err", err)
		os.Exit(1)
	}
	log.Info("database migration complete", "provider", cfg.Database.Provider)
}
