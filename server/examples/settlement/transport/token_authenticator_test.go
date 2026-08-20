package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestTokenAuthenticatorAcceptsOnlyStrongMatchingBearerToken(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	digest := sha256.Sum256([]byte(token))
	authenticator, err := NewTokenAuthenticator(hex.EncodeToString(digest[:]), "settlement-client", "org-1")
	if err != nil {
		t.Fatalf("NewTokenAuthenticator() error = %v", err)
	}
	principal, err := authenticator.AuthenticateAPIToken(context.Background(), token)
	if err != nil {
		t.Fatalf("AuthenticateAPIToken() error = %v", err)
	}
	if principal.OrganizationID != "org-1" || !principal.HasPermission(PermissionReadSettlement) {
		t.Fatalf("principal = %#v", principal)
	}
	for _, invalid := range []string{"too-short", "0123456789abcdef0123456789abcdeg"} {
		if _, err := authenticator.AuthenticateAPIToken(context.Background(), invalid); err == nil {
			t.Fatalf("AuthenticateAPIToken() accepted %q", invalid)
		}
	}
	if _, err := authenticator.Authenticate(context.Background(), token); err == nil {
		t.Fatal("Authenticate() accepted a browser session")
	}
}
