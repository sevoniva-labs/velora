package config

import (
	"strings"
	"testing"
)

func TestTurnstileConfigurationRequiresCompleteValues(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Security.TurnstileSiteKey = "site-key"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "VELORA_TURNSTILE_SECRET") {
		t.Fatalf("Validate() error = %v, want missing Turnstile secret", err)
	}
}

func TestTurnstileConfigurationDefaultsActionAndDetectsEnabled(t *testing.T) {
	cfg := Default()
	cfg.Security.TurnstileSiteKey = "site-key"
	cfg.Security.TurnstileSecret = "secret"
	cfg.Security.TurnstileHostnames = []string{"localhost"}
	if !cfg.Security.TurnstileConfigured() {
		t.Fatal("TurnstileConfigured() = false, want true")
	}
	if got := cfg.Security.EffectiveTurnstileAction(); got != "login" {
		t.Fatalf("EffectiveTurnstileAction() = %q, want login", got)
	}
}

func TestProductionCasdoorPasswordLoginRequiresTurnstile(t *testing.T) {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Security.AuthMode = "oidc"
	cfg.Security.CasdoorPasswordLoginEnabled = true
	if err := cfg.ValidateProductionAuth(); err == nil || !strings.Contains(err.Error(), "requires Turnstile configuration") {
		t.Fatalf("ValidateProductionAuth() error = %v, want Turnstile requirement", err)
	}
}
