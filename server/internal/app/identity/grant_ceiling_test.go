package identity

import (
	"errors"
	"testing"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestAuthorizeGrantActor(t *testing.T) {
	verifiedAt := time.Now().UTC()
	valid := domain.Principal{Type: "USER", UserID: "u1", OrganizationID: "org1", MFAVerifiedAt: &verifiedAt}
	if err := authorizeGrantActor(valid, "org1"); err != nil {
		t.Fatalf("valid user actor rejected: %v", err)
	}
	if err := authorizeGrantActor(domain.Principal{Type: "USER", UserID: "u1", OrganizationID: "org1"}, "org1"); !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("missing MFA should require step-up, got %v", err)
	}
	expired := time.Now().UTC().Add(-recentMFAWindow - time.Second)
	if err := authorizeGrantActor(domain.Principal{Type: "USER", UserID: "u1", OrganizationID: "org1", MFAVerifiedAt: &expired}, "org1"); !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("expired MFA should require step-up, got %v", err)
	}
	for name, actor := range map[string]domain.Principal{
		"token":        {Type: "TOKEN", UserID: "u1", OrganizationID: "org1"},
		"missing user": {Type: "USER", OrganizationID: "org1"},
		"cross org":    {Type: "USER", UserID: "u1", OrganizationID: "org2"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := authorizeGrantActor(actor, "org1"); !errors.Is(err, ErrGrantCeiling) {
				t.Fatalf("expected grant ceiling error, got %v", err)
			}
		})
	}
}

func TestAuthorizeDirectoryActorDoesNotForceMFA(t *testing.T) {
	valid := domain.Principal{Type: "USER", UserID: "u1", OrganizationID: "org1"}
	if err := authorizeDirectoryActor(valid, "org1"); err != nil {
		t.Fatalf("interactive directory actor rejected without MFA: %v", err)
	}
	for name, actor := range map[string]domain.Principal{
		"token":        {Type: "TOKEN", UserID: "u1", OrganizationID: "org1"},
		"missing user": {Type: "USER", OrganizationID: "org1"},
		"cross org":    {Type: "USER", UserID: "u1", OrganizationID: "org2"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := authorizeDirectoryActor(actor, "org1"); !errors.Is(err, ErrGrantCeiling) {
				t.Fatalf("expected grant ceiling error, got %v", err)
			}
		})
	}
}

func TestEnforceRoleMutation(t *testing.T) {
	systemAdmin := domain.Principal{Roles: []string{"system_admin"}}
	if err := enforceRoleMutation(systemAdmin, nil, []string{"system_admin", "security_admin"}); err != nil {
		t.Fatalf("system admin rejected: %v", err)
	}

	delegated := domain.Principal{Roles: []string{"security_admin", "user"}}
	if err := enforceRoleMutation(delegated, []string{"user"}, []string{"user", "security_admin"}); err != nil {
		t.Fatalf("owned role grant rejected: %v", err)
	}
	if err := enforceRoleMutation(delegated, []string{"user"}, []string{"user", "system_admin"}); !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("higher role grant should fail, got %v", err)
	}
	if err := enforceRoleMutation(delegated, []string{"system_admin", "user"}, []string{"user"}); !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("higher role removal should fail, got %v", err)
	}
}

func TestEnforcePermissionMutation(t *testing.T) {
	systemAdmin := domain.Principal{Roles: []string{"system_admin"}}
	if err := enforcePermissionMutation(systemAdmin, "security_admin", nil, []string{"system.user.update"}); err != nil {
		t.Fatalf("system admin rejected: %v", err)
	}

	delegated := domain.Principal{
		Roles:       []string{"security_admin"},
		Permissions: []string{"system.user.read", "system.user.update"},
	}
	if err := enforcePermissionMutation(delegated, "security_admin", []string{"system.user.read"}, []string{"system.user.read", "system.user.update"}); err != nil {
		t.Fatalf("owned permission change rejected: %v", err)
	}
	if err := enforcePermissionMutation(delegated, "auditor", nil, []string{"system.user.read"}); !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("unowned role mutation should fail, got %v", err)
	}
	if err := enforcePermissionMutation(delegated, "security_admin", nil, []string{"system.role.manage"}); !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("unowned permission grant should fail, got %v", err)
	}
	if err := enforcePermissionMutation(delegated, "security_admin", []string{"system.role.manage"}, nil); !errors.Is(err, ErrGrantCeiling) {
		t.Fatalf("unowned permission removal should fail, got %v", err)
	}
}
