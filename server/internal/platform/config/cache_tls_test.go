package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresRedisTLSInProduction(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=verify-full"
	cfg.Cache.Provider = "redis"
	cfg.Cache.Addresses = []string{"redis.internal:6379"}
	cfg.Cache.TLS = false

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "cache.tls must be enabled") {
		t.Fatalf("production Redis without TLS should be rejected, got %v", err)
	}
	cfg.Cache.TLS = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production Redis with TLS rejected: %v", err)
	}
	cfg.App.Environment = "development"
	cfg.Cache.TLS = false
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development Redis without TLS unexpectedly rejected: %v", err)
	}
}
