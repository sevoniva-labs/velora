package config

import (
	"strings"
	"testing"
)

func TestValidateGovernsWebCSPFrameSources(t *testing.T) {
	tests := []struct {
		name   string
		source []string
		want   string
	}{
		{name: "plaintext production origin", source: []string{"http://remote.example.cn"}, want: "exact HTTP(S) origins"},
		{name: "wildcard", source: []string{"https://*.example.cn"}, want: "exact HTTP(S) origins"},
		{name: "path", source: []string{"https://remote.example.cn/app"}, want: "exact HTTP(S) origins"},
		{name: "duplicate", source: []string{"https://remote.example.cn", "https://remote.example.cn"}, want: "must not contain duplicates"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := productionConfig()
			cfg.Server.WebCSPFrameSources = tt.source
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error=%v want=%q", err, tt.want)
			}
		})
	}

	cfg := productionConfig()
	cfg.Server.WebCSPFrameSources = []string{"https://remote.example.cn"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid production frame source rejected: %v", err)
	}
}

func TestValidateRequiresWujieDeploymentApprovalReference(t *testing.T) {
	cfg := productionConfig()
	cfg.Server.WebCSPWujieEnabled = true
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "web_csp_wujie_approval_ref") {
		t.Fatalf("Wujie without approval reference should fail, got %v", err)
	}
	cfg.Server.WebCSPWujieApprovalRef = "SEC-2026-001"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("approved Wujie CSP rejected: %v", err)
	}
}

func TestApplyEnvironmentLoadsWebCSPControls(t *testing.T) {
	t.Setenv("VELORA_WEB_CSP_FRAME_SOURCES", "https://one.example.cn,https://two.example.cn")
	t.Setenv("VELORA_WEB_CSP_WUJIE_ENABLED", "true")
	t.Setenv("VELORA_WEB_CSP_WUJIE_APPROVAL_REF", "SEC-2026-002")
	cfg := Default()
	ApplyEnvironment(&cfg)
	if len(cfg.Server.WebCSPFrameSources) != 2 || !cfg.Server.WebCSPWujieEnabled || cfg.Server.WebCSPWujieApprovalRef != "SEC-2026-002" {
		t.Fatalf("web CSP environment controls not applied: %+v", cfg.Server)
	}
}
