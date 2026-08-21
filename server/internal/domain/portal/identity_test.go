package portal

import "testing"

func TestIdentityBindingValidationAllowsOnlyLoopbackHTTPInDevelopment(t *testing.T) {
	base := IdentityBindingInput{ProviderKey: IdentityProviderCasdoor, Protocol: ProtocolOIDC, ProviderApplicationRef: "demo", PublicClientID: "client", Issuer: "http://localhost:8443", RedirectURIs: []string{"http://localhost:5173/auth/callback"}}
	if err := base.Validate(); err != nil {
		t.Fatalf("loopback development URLs should be accepted: %v", err)
	}
	base.Issuer = "http://identity.example.com"
	if err := base.Validate(); err == nil {
		t.Fatal("non-loopback HTTP issuer should be rejected")
	}
}

func TestIdentityBindingValidationRejectsUnverifiedProtocols(t *testing.T) {
	base := IdentityBindingInput{ProviderKey: IdentityProviderCasdoor, ProviderApplicationRef: "demo", PublicClientID: "client", Issuer: "https://identity.example.test", RedirectURIs: []string{"https://app.example.test/auth/callback"}}
	for _, protocol := range []string{ProtocolSAML, ProtocolCAS, ProtocolForwardAuth} {
		base.Protocol = protocol
		if err := base.Validate(); err == nil {
			t.Fatalf("protocol %s: Validate() error = nil, want rejection", protocol)
		}
	}
}

func TestIdentityBindingScopesNormalizeAndRejectUnsafeValues(t *testing.T) {
	base := IdentityBindingInput{ProviderKey: IdentityProviderCasdoor, Protocol: ProtocolOIDC, ProviderApplicationRef: "demo", PublicClientID: "client", Issuer: "https://identity.example.test", RedirectURIs: []string{"https://app.example.test/auth/callback"}}
	got, err := NormalizeOIDCScopes([]string{"openid profile", "email", "profile"})
	if err != nil || len(got) != 3 || got[0] != "openid" || got[1] != "profile" || got[2] != "email" {
		t.Fatalf("NormalizeOIDCScopes() = %#v, %v", got, err)
	}
	base.Scopes = []string{"openid", "profile", "email"}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid scopes rejected: %v", err)
	}
	for _, scopes := range [][]string{{"openid,profile"}, {"openid\nprofile"}, {"openid", "bad*scope"}} {
		base.Scopes = scopes
		if err := base.Validate(); err == nil {
			t.Fatalf("unsafe scopes %#v were accepted", scopes)
		}
	}
}
