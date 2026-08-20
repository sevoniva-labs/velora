package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func validProductionConfig() *Config {
	return &Config{
		Env:                 "production",
		PublicBaseURL:       "https://velora.example.com",
		CasdoorIssuer:       "https://velora.example.com/casdoor",
		CasdoorClientID:     "velora",
		CasdoorClientSecret: "client-secret",
		CasdoorRedirectURI:  "https://velora.example.com/api/v1/auth/oidc/callback",
		DatabaseURL:         "postgres://velora_app:secret@postgres:5432/velora?sslmode=require",
		RedisURL:            "rediss://:secret@redis:6379/0",
		SessionSecret:       strings.Repeat("s", 32),
		MailCredentialKey:   base64.StdEncoding.EncodeToString([]byte(strings.Repeat("m", 32))),
		SessionTTL:          time.Hour,
	}
}

func TestValidateProductionConfig(t *testing.T) {
	if err := validProductionConfig().validate(); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}
}

func TestValidateProductionConfigRejectsUnsafeDefaults(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"public localhost", func(c *Config) { c.PublicBaseURL = "http://localhost:8080" }},
		{"issuer mismatch", func(c *Config) { c.CasdoorIssuer = "https://casdoor.example.com" }},
		{"redirect mismatch", func(c *Config) { c.CasdoorRedirectURI = "https://evil.example.com/callback" }},
		{"redis missing", func(c *Config) { c.RedisURL = "" }},
		{"redis localhost", func(c *Config) { c.RedisURL = "redis://localhost:6379/0" }},
		{"mail key missing", func(c *Config) { c.MailCredentialKey = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validProductionConfig()
			tc.mutate(c)
			if err := c.validate(); err == nil {
				t.Fatal("expected production configuration validation error")
			}
		})
	}
}

func TestValidateRedisURL(t *testing.T) {
	for _, raw := range []string{"redis://redis:6379/0", "rediss://:secret@redis:6379/0"} {
		if err := validateRedisURL(raw); err != nil {
			t.Errorf("%q rejected: %v", raw, err)
		}
	}
	for _, raw := range []string{"http://redis:6379", "redis://localhost:6379/0", "redis://"} {
		if err := validateRedisURL(raw); err == nil {
			t.Errorf("%q unexpectedly accepted", raw)
		}
	}
}
