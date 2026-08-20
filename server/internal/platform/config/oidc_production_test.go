package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateProductionAuthRequiresOIDCConfiguration(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	if err := cfg.ValidateProductionAuth(); err == nil || !strings.Contains(err.Error(), "auth_mode must be oidc") {
		t.Fatalf("missing production OIDC mode was accepted: %v", err)
	}
	cfg.Security.AuthMode = "oidc"
	cfg.Security.OIDCIssuer = "https://casdoor.example.test"
	cfg.Security.OIDCClientID = "velora"
	cfg.Security.OIDCClientSecret = "secret"
	cfg.Security.OIDCRedirectURL = "https://velora.example.test/auth/callback"
	cfg.Security.SessionTTL = time.Hour
	cfg.Security.CasdoorAccountURL = "https://casdoor.example.test/account"
	if err := cfg.ValidateProductionAuth(); err != nil {
		t.Fatalf("valid production OIDC configuration rejected: %v", err)
	}
}

func TestValidateProductionAuthRejectsVeloraOIDCProvider(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Security.AuthMode = "oidc"
	cfg.Security.OIDCIssuer = "https://casdoor.example.test"
	cfg.Security.OIDCClientID = "velora"
	cfg.Security.OIDCClientSecret = "secret"
	cfg.Security.OIDCRedirectURL = "https://velora.example.test/auth/callback"
	cfg.Security.SessionTTL = time.Hour
	cfg.Security.CasdoorAccountURL = "https://casdoor.example.test/account"
	cfg.Security.OIDCProviderEnabled = true
	if err := cfg.ValidateProductionAuth(); err == nil || !strings.Contains(err.Error(), "oidc_provider_enabled must be false") {
		t.Fatalf("self-hosted OIDC provider was not rejected: %v", err)
	}
}

func TestValidateProductionAuthRejectsSoftwareGMTrustRoot(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Security.AuthMode = "oidc"
	cfg.Security.OIDCIssuer = "https://casdoor.example.test"
	cfg.Security.OIDCClientID = "velora"
	cfg.Security.OIDCClientSecret = "secret"
	cfg.Security.OIDCRedirectURL = "https://velora.example.test/auth/callback"
	cfg.Security.SessionTTL = time.Hour
	cfg.Security.CasdoorAccountURL = "https://casdoor.example.test/account"
	cfg.Security.CryptoProvider = "gm"
	cfg.Security.CryptoAdapter = "software"
	if err := cfg.ValidateProductionAuth(); err == nil || !strings.Contains(err.Error(), "requires an approved KMS/HSM/PKCS#11 adapter") {
		t.Fatalf("software GM trust root was not rejected: %v", err)
	}
}

func TestValidateProductionTransportSecurity(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Database.DSN = "postgres://velora:secret@db/velora?sslmode=verify-full"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "server.public_url must be a non-loopback https URL") {
		t.Fatalf("insecure production public URL was not rejected: %v", err)
	}

	cfg.Server.PublicURL = "https://velora.example.test"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "security.secure_cookies must be true") {
		t.Fatalf("insecure production cookies were not rejected: %v", err)
	}

	cfg.Security.SecureCookies = true
	cfg.Security.AllowedOrigins = []string{"https://velora.example.test"}
	cfg.Cache.Provider = "redis"
	cfg.Cache.Addresses = []string{"redis.internal:6379"}
	cfg.Cache.TLS = true
	cfg.Storage.Provider = "s3-compatible"
	cfg.Storage.Endpoint = "https://objects.internal"
	cfg.Storage.Bucket = "velora"
	cfg.Storage.TLS = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid production transport security rejected: %v", err)
	}

	cfg.Security.AllowedOrigins = []string{"*"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "security.allowed_origins must contain exact https origins") {
		t.Fatalf("wildcard production origin was not rejected: %v", err)
	}
}

func TestValidateProductionRequiresSharedRedisAndObjectStorage(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Database.DSN = "postgres://velora:secret@db/velora?sslmode=verify-full"
	cfg.Server.PublicURL = "https://velora.example.test"
	cfg.Security.SecureCookies = true
	cfg.Security.AllowedOrigins = []string{"https://velora.example.test"}
	cfg.Storage.LocalRoot = "/var/lib/velora"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "cache.provider must be redis") || !strings.Contains(err.Error(), "storage.provider must be an S3-compatible target") {
		t.Fatalf("production accepted local/shared-state fallbacks: %v", err)
	}
}
