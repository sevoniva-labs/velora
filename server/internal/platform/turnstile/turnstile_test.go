package turnstile

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testVerifier(t *testing.T, body string, status int) (*Verifier, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
			t.Errorf("content type = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("secret") != "secret" || r.Form.Get("response") != "token" || r.Form.Get("remoteip") != "192.0.2.10" {
			t.Errorf("unexpected siteverify form: %#v", r.Form)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	verifier := &Verifier{secret: "secret", action: "login", hostnames: map[string]struct{}{"localhost": {}}, client: server.Client(), endpoint: server.URL}
	return verifier, server
}

func TestVerifyRequiresSuccessActionAndHostname(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		wantOK bool
	}{
		{name: "success", body: `{"success":true,"action":"login","hostname":"localhost"}`, wantOK: true},
		{name: "wrong action", body: `{"success":true,"action":"signup","hostname":"localhost"}`},
		{name: "wrong hostname", body: `{"success":true,"action":"login","hostname":"evil.example"}`},
		{name: "provider rejected", body: `{"success":false,"error-codes":["invalid-input-response"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, server := testVerifier(t, tt.body, http.StatusOK)
			defer server.Close()
			err := verifier.Verify(context.Background(), "token", "192.0.2.10")
			if (err == nil) != tt.wantOK {
				t.Fatalf("Verify() error = %v, want success=%v", err, tt.wantOK)
			}
		})
	}
}

func TestVerifyFailsClosedOnUpstreamErrorAndOversizedToken(t *testing.T) {
	verifier, server := testVerifier(t, `{}`, http.StatusBadGateway)
	defer server.Close()
	if err := verifier.Verify(context.Background(), "token", "192.0.2.10"); err == nil {
		t.Fatal("Verify() accepted non-200 siteverify response")
	}
	if err := verifier.Verify(context.Background(), strings.Repeat("x", 2049), "192.0.2.10"); err == nil {
		t.Fatal("Verify() accepted oversized token")
	}
}
