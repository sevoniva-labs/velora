package tlsx

import (
	"crypto/tls"
	"testing"
)

func TestClientConfigUsesVerifiedTLS12OrNewer(t *testing.T) {
	cfg, err := ClientConfig(ClientOptions{Enabled: true, ServerName: "redis.internal.example"})
	if err != nil {
		t.Fatalf("ClientConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("ClientConfig() returned nil for enabled TLS")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %d, want TLS 1.2", cfg.MinVersion)
	}
	if cfg.InsecureSkipVerify {
		t.Fatal("certificate verification must not be disabled")
	}
	if cfg.ServerName != "redis.internal.example" {
		t.Fatalf("ServerName = %q", cfg.ServerName)
	}
}
