package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProductionProfilesAreSecureAndValid(t *testing.T) {
	profiles := []string{"standard.yaml", "full.yaml", "xinchuang.yaml"}
	for _, profile := range profiles {
		t.Run(profile, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "configs", profile))
			if err != nil {
				t.Fatalf("read profile: %v", err)
			}
			cfg := Default()
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				t.Fatalf("parse profile: %v", err)
			}
			switch cfg.Database.Provider {
			case "postgres":
				cfg.Database.DSN = "postgres://velora:secret@db/forge?sslmode=verify-full"
			case "oceanbase":
				cfg.Database.DSN = "velora:secret@tcp(oceanbase:2881)/forge?tls=true"
			}
			if cfg.Messaging.Provider == "rocketmq" {
				cfg.Messaging.RocketMQAccessKey = "access-key"
				cfg.Messaging.RocketMQSecretKey = "secret-key"
			}
			if cfg.Storage.Provider != "local" {
				cfg.Storage.AccessKey = "access-key"
				cfg.Storage.SecretKey = "secret-key"
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("production profile rejected: %v", err)
			}
			if cfg.Database.AutoMigrate {
				t.Fatal("production profile enables database auto-migration")
			}
			if profile == "full.yaml" && (cfg.Messaging.Provider != "rocketmq" || cfg.Streaming.Provider != "disabled") {
				t.Fatalf("full profile messaging=%q streaming=%q", cfg.Messaging.Provider, cfg.Streaming.Provider)
			}
		})
	}
}

func TestProductionRejectsDatabaseAutoMigration(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Database.AutoMigrate = true
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "database.auto_migrate must be false") {
		t.Fatalf("production auto migration should be rejected, got %v", err)
	}
}
