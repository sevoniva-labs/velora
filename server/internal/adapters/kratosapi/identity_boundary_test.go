package kratosapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdentityConnectionStatusVerifiesDiscoveryIssuer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"https://identity.example.test"}`))
	}))
	defer server.Close()

	if got := identityConnectionStatus(context.Background(), "https://identity.example.test", server.URL); got != "CONNECTED" {
		t.Fatalf("connected status = %q", got)
	}
	if got := identityConnectionStatus(context.Background(), "https://other.example.test", server.URL); got != "MISMATCH" {
		t.Fatalf("mismatch status = %q", got)
	}
	if got := identityConnectionStatus(context.Background(), "", server.URL); got != "UNCONFIGURED" {
		t.Fatalf("empty issuer status = %q", got)
	}
}
