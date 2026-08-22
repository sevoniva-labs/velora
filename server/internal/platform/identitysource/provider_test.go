package identitysource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"golang.org/x/oauth2"
)

func TestNewLDAPProviderRequiresSecureTransportByDefault(t *testing.T) {
	_, err := NewLDAPProvider(LDAPConfig{Name: "ad", URL: "ldap://directory.example", BindDN: "cn=svc", BaseDN: "dc=example,dc=com", LoginAttribute: "sAMAccountName"})
	if err != ErrInvalidConfiguration {
		t.Fatalf("NewLDAPProvider() error = %v", err)
	}
}

func TestNewLDAPProviderAppliesSafeDefaults(t *testing.T) {
	provider, err := NewLDAPProvider(LDAPConfig{Name: "ad", URL: "ldaps://directory.example", BindDN: "cn=svc", BaseDN: "dc=example,dc=com", LoginAttribute: "sAMAccountName"})
	if err != nil {
		t.Fatalf("NewLDAPProvider() error = %v", err)
	}
	if provider.cfg.SearchTimeout != 5*time.Second || provider.cfg.DisplayAttribute != "displayName" || provider.cfg.GroupAttribute != "memberOf" {
		t.Fatalf("unexpected LDAP defaults: %#v", provider.cfg)
	}
	filter := fmt.Sprintf(provider.cfg.UserFilter, ldap.EscapeFilter(provider.cfg.LoginAttribute), ldap.EscapeFilter("alice"))
	if filter != "(&(objectClass=person)(sAMAccountName=alice))" {
		t.Fatalf("LDAP user filter = %q", filter)
	}
}

func TestUniqueStringsRemovesEmptyAndDuplicates(t *testing.T) {
	got := uniqueStrings([]string{" user ", "", "user", "auditor"})
	if len(got) != 2 || got[0] != "user" || got[1] != "auditor" {
		t.Fatalf("uniqueStrings() = %#v", got)
	}
}

func TestClaimsIndicateMFAAcceptsStandardAMRAndACRForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		acr  string
		want bool
	}{
		{name: "array", raw: `["pwd","mfa"]`, want: true},
		{name: "string", raw: `"totp"`, want: true},
		{name: "acr", raw: `[]`, acr: "urn:example:mfa", want: true},
		{name: "password only", raw: `["pwd"]`, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := claimsIndicateMFA([]byte(tc.raw), tc.acr); got != tc.want {
				t.Fatalf("claimsIndicateMFA(%s, %q) = %v, want %v", tc.raw, tc.acr, got, tc.want)
			}
		})
	}
}

func TestNewOIDCProviderRequiresConfidentialClientConfiguration(t *testing.T) {
	_, err := NewOIDCProvider(context.Background(), nil, OIDCConfig{
		Name: "casdoor", Issuer: "https://casdoor.example.com", ClientID: "velora", RedirectURL: "https://velora.example.com/api/v1/auth/federated/oidc/casdoor/callback",
	})
	if !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("NewOIDCProvider() error = %v, want ErrInvalidConfiguration", err)
	}
}

func TestEndSessionURLUsesExplicitBrowserBridge(t *testing.T) {
	provider := &OIDCProvider{
		config:    oauth2.Config{ClientID: "velora"},
		logoutURL: "https://casdoor.example.com/_velora/logout",
	}
	got, err := provider.EndSessionURL()
	if err != nil {
		t.Fatalf("EndSessionURL() error = %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "casdoor.example.com" {
		t.Fatalf("unexpected logout URL %q", got)
	}
	if parsed.Path != "/_velora/logout" || parsed.RawQuery != "" {
		t.Fatalf("logout URL = %v", parsed)
	}
}

func TestEndSessionURLIsOptionalWhenBridgeIsNotConfigured(t *testing.T) {
	provider := &OIDCProvider{config: oauth2.Config{ClientID: "velora"}, endSessionEndpoint: "https://casdoor.example.com/api/logout"}
	got, err := provider.EndSessionURL()
	if err != nil || got != "" {
		t.Fatalf("EndSessionURL() = %q, %v; want empty optional URL", got, err)
	}
}

func TestOIDCJSONTokenExchangePreservesIDToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","id_token":"id","token_type":"Bearer","expires_in":60}`))
	}))
	defer server.Close()
	provider := &OIDCProvider{
		config:     oauth2.Config{ClientID: "client", ClientSecret: "secret", RedirectURL: "https://velora.example/callback"},
		tokenURL:   server.URL,
		httpClient: server.Client(),
	}
	token, err := provider.exchangeJSON(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access" || token.Extra("id_token") != "id" || token.TokenType != "Bearer" {
		t.Fatalf("unexpected token: %#v", token)
	}
}

func TestOIDCJSONTokenExchangeFailsClosedOnProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"rejected"}`))
	}))
	defer server.Close()
	provider := &OIDCProvider{config: oauth2.Config{ClientID: "client", ClientSecret: "secret"}, tokenURL: server.URL, httpClient: server.Client()}
	if _, err := provider.exchangeJSON(context.Background(), "code", "verifier"); err == nil {
		t.Fatal("provider error was accepted")
	}
}

func TestInternalURLRoundTripperRewritesIssuerRequests(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer internal.Close()
	external, _ := url.Parse("https://casdoor.example.com")
	internalURL, _ := url.Parse(internal.URL)
	transport := &internalURLRoundTripper{base: http.DefaultTransport, external: external, internal: internalURL}
	client := &http.Client{Transport: transport}
	resp, err := client.Get("https://casdoor.example.com/.well-known/openid-configuration")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
