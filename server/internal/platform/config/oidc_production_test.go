package config

import (
	"strings"
	"testing"
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
	cfg.Security.CasdoorAccountURL = "https://casdoor.example.test/account"
	cfg.Security.OIDCProviderEnabled = true
	if err := cfg.ValidateProductionAuth(); err == nil || !strings.Contains(err.Error(), "oidc_provider_enabled must be false") {
		t.Fatalf("self-hosted OIDC provider was not rejected: %v", err)
	}
}
