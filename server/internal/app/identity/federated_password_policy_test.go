package identity

import (
	"testing"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestApplyPasswordRequirementFederatedSessionIgnoresLocalPassword(t *testing.T) {
	service := &Service{}
	principal := domain.Principal{
		AuthenticationLevel: "FEDERATED",
		MustChangePassword:  true,
		PasswordChangedAt:   time.Now().UTC().Add(-365 * 24 * time.Hour),
	}

	service.applyPasswordRequirement(&principal, 90*24*time.Hour)

	if principal.MustChangePassword {
		t.Fatal("federated session must not be blocked by the local password policy")
	}
}

func TestApplyPasswordRequirementLocalSessionStillEnforcesExpiry(t *testing.T) {
	service := &Service{}
	principal := domain.Principal{
		AuthenticationLevel: "PASSWORD",
		PasswordChangedAt:   time.Now().UTC().Add(-91 * 24 * time.Hour),
	}

	service.applyPasswordRequirement(&principal, 90*24*time.Hour)

	if !principal.MustChangePassword {
		t.Fatal("expired local password must still require a password change")
	}
}
