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
