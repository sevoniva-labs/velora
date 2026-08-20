package httpserver

import (
	"strings"
	"testing"
)

func TestWebContentSecurityPolicySeparatesConnectAndFrameSources(t *testing.T) {
	policy := webContentSecurityPolicy("test-nonce", SPAOptions{
		ConnectSources: []string{"https://api.example.com"},
		FrameSources:   []string{"https://frame.example.com"},
	})

	if !strings.Contains(policy, "connect-src 'self' https://api.example.com") {
		t.Fatalf("connect source missing from policy: %s", policy)
	}
	if strings.Contains(policy, "connect-src 'self' https://frame.example.com") {
		t.Fatalf("frame source leaked into connect policy: %s", policy)
	}
	if !strings.Contains(policy, "frame-src https://frame.example.com") {
		t.Fatalf("frame source missing from policy: %s", policy)
	}
}
