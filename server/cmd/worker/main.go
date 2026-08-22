package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/idempotency"
	"github.com/sevoniva-labs/velora/server/internal/platform/logx"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
	"github.com/sevoniva-labs/velora/server/internal/platform/provisioninghttp"
	"github.com/sevoniva-labs/velora/server/internal/platform/reliablemsg"
)

var version = "0.2.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logx.New(cfg.Observability.LogLevel, cfg.Observability.LogFormat, cfg.App.Name+"-worker", cfg.App.Environment, version)
	slog.SetDefault(log)
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	bus, err := messaging.New(cfg.Messaging)
	if err != nil {
		return err
	}
	defer bus.Close()
	provisioning, err := provisioninghttp.New(provisioninghttp.Config{Enabled: cfg.Provisioning.SpectraEnabled, URL: cfg.Provisioning.SpectraURL, Secret: cfg.Provisioning.SpectraSecret})
	if err != nil {
		return err
	}
	if bus.Provider() == "disabled" && !provisioning.Enabled() {
		log.Info("reliable-message worker has no enabled provider")
		<-ctx.Done()
		return nil
	}
	messages := reliablemsg.New(db)
	idem := idempotency.New(db)
	auditWriter := audit.NewWriter(db)
	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	gc := time.NewTicker(time.Hour)
	defer gc.Stop()
	auditGC := time.NewTicker(24 * time.Hour)
	defer auditGC.Stop()
	runAuditRetention := func() {
		if cfg.Compliance.AuditRetentionDays <= 0 {
			return
		}
		if n, err := auditWriter.PurgeExpired(ctx, cfg.Compliance.AuditRetentionDays); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("audit log retention gc", "err", err, "retention_days", cfg.Compliance.AuditRetentionDays)
		} else if n > 0 {
			log.Info("audit logs purged", "deleted", n, "retention_days", cfg.Compliance.AuditRetentionDays)
		}
	}
	runAuditRetention()
	if cfg.Compliance.NetworkLogRetentionDays > 0 {
		log.Info("network log retention is enforced by your log platform in this scaffold; app retention config kept for policy control")
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-gc.C:
			if err := idem.PurgeExpired(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Error("idempotency gc", "err", err)
			}
		case <-auditGC.C:
			runAuditRetention()
		case <-poll.C:
			if provisioning.Enabled() {
				n, err := messages.PublishTopicBatch(ctx, "velora.provisioning.spectra", 50, provisioning.Publish)
				if err != nil && !errors.Is(err, context.Canceled) {
					log.Error("Spectra provisioning publish", "err", err)
				} else if n > 0 {
					log.Info("Spectra provisioning published", "count", n)
				}
			}
			if bus.Provider() != "disabled" {
				n, err := messages.PublishBatch(ctx, bus, 100)
				if err != nil && !errors.Is(err, context.Canceled) {
					log.Error("reliable message publish", "err", err)
				} else if n > 0 {
					log.Info("reliable message published", "count", n)
				}
			}
		}
	}
}
