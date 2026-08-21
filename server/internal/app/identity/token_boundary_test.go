package identity

import (
	"errors"
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestRequireInteractivePrincipal(t *testing.T) {
	valid := domain.Principal{Type: "USER", UserID: "u1", OrganizationID: "org1", SessionID: "session1"}
	if err := requireInteractivePrincipal(valid); err != nil {
		t.Fatalf("interactive principal rejected: %v", err)
	}
	tests := map[string]domain.Principal{
		"API token":       {Type: "TOKEN", UserID: "u1", OrganizationID: "org1"},
		"missing user":    {Type: "USER", OrganizationID: "org1", SessionID: "session1"},
		"missing org":     {Type: "USER", UserID: "u1", SessionID: "session1"},
		"missing session": {Type: "USER", UserID: "u1", OrganizationID: "org1"},
	}
	for name, principal := range tests {
		t.Run(name, func(t *testing.T) {
			if err := requireInteractivePrincipal(principal); !errors.Is(err, ErrInteractiveSessionRequired) {
				t.Fatalf("expected interactive-session error, got %v", err)
			}
		})
	}
}
