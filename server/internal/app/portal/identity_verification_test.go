package portal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

func TestVerifyBindingRejectsIssuerOutsideConfiguredCasdoor(t *testing.T) {
	binding := portaldomain.IdentityBinding{Protocol: portaldomain.ProtocolOIDC, Issuer: "https://attacker.example"}
	passed, _, code, _ := verifyBinding(context.Background(), binding, "https://auth.example")
	if passed || code != "ISSUER_NOT_ALLOWED" {
		t.Fatalf("passed=%v code=%q", passed, code)
	}
}

func TestVerifyBindingAcceptsExactIssuerWithoutFollowingRedirects(t *testing.T) {
	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + issuer + `"}`))
	}))
	defer server.Close()
	issuer = server.URL

	binding := portaldomain.IdentityBinding{Protocol: portaldomain.ProtocolOIDC, Issuer: issuer}
	passed, _, code, _ := verifyBinding(context.Background(), binding, issuer)
	if !passed || code != "" {
		t.Fatalf("passed=%v code=%q", passed, code)
	}

	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("verification followed a redirect")
	}))
	defer redirectTarget.Close()
	redirecting := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer redirecting.Close()
	binding.Issuer = redirecting.URL
	passed, _, code, _ = verifyBinding(context.Background(), binding, redirecting.URL)
	if passed || code != "DISCOVERY_HTTP_302" {
		t.Fatalf("redirect passed=%v code=%q", passed, code)
	}
}
