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
