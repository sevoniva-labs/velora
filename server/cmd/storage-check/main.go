// Command storage-check performs a read-only object-store connectivity and
// capability check for a deployment target. It never creates or deletes an
// object and never treats a generic S3 profile as vendor certification.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/storage"
)

type report struct {
	Provider     string                                          `json:"provider"`
	Profile      storage.ProviderProfile                         `json:"profile"`
	Capabilities map[storage.Capability]storage.CapabilityStatus `json:"capabilities"`
}

func main() {
	timeout := 30 * time.Second
	if raw := strings.TrimSpace(os.Getenv("VELORA_STORAGE_CHECK_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 || parsed > 5*time.Minute {
			fail(fmt.Errorf("VELORA_STORAGE_CHECK_TIMEOUT must be between 1s and 5m: %w", err))
		}
		timeout = parsed
	}

	cfg, err := config.Load()
	if err != nil {
		fail(fmt.Errorf("config: %w", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	store, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		fail(fmt.Errorf("storage: %w", err))
	}
	if err := store.Ping(ctx); err != nil {
		fail(fmt.Errorf("storage ping: %w", err))
	}
	reporter, ok := store.(storage.CapabilityReporter)
	if !ok {
		fail(errors.New("storage provider does not report capabilities"))
	}
	profileReporter, ok := store.(storage.ProfileReporter)
	if !ok {
		fail(errors.New("storage provider does not report its provider profile"))
	}
	capabilities := reporter.Capabilities()
	if raw := strings.TrimSpace(os.Getenv("VELORA_STORAGE_CHECK_REQUIRE")); raw != "" {
		required := make([]storage.Capability, 0)
		for _, item := range strings.Split(raw, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			required = append(required, storage.Capability(item))
		}
		if err := storage.RequireTargetTestedCapabilities(store, required...); err != nil {
			fail(fmt.Errorf("required target-tested capability: %w", err))
		}
	}
	output := report{Provider: store.Provider(), Profile: profileReporter.Profile(), Capabilities: capabilities}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(fmt.Errorf("encode report: %w", err))
	}
}

func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "storage-check failed: %v\n", err)
	os.Exit(1)
}
