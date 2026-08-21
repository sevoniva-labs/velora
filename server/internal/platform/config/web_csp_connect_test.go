package config

import (
	"strings"
	"testing"
)

func TestValidateGovernsWebCSPConnectSources(t *testing.T) {
	cfg := Config{}
	cfg.App.Environment = "production"
	cfg.Server.WebCSPConnectSources = []string{"http://remote.example.com"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "server.web_csp_connect_sources must contain exact HTTP(S) origins and use HTTPS in production") {
		t.Fatalf("expected production HTTPS validation error, got %v", err)
	}

	cfg.Server.WebCSPConnectSources = []string{"https://remote.example.com", "https://remote.example.com"}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "server.web_csp_connect_sources must not contain duplicates") {
		t.Fatalf("expected duplicate source validation error, got %v", err)
	}
}

func TestValidWebCSPOriginRejectsPathsAndWildcards(t *testing.T) {
	for _, source := range []string{
		"https://*.example.com",
		"https://remote.example.com/api",
		"https://user@remote.example.com",
		"https://remote.example.com?tenant=1",
	} {
		if validWebCSPOrigin(source, true) {
			t.Fatalf("source %q must be rejected", source)
		}
	}

	if !validWebCSPOrigin("https://remote.example.com:8443", true) {
		t.Fatal("exact HTTPS origin with an explicit port must be accepted")
	}
}
