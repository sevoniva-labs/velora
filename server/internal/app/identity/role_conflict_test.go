package identity

import (
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestHasRoleConflict(t *testing.T) {
	rules := []domain.RoleConflictRule{{RoleA: "system_admin", RoleB: "auditor"}}
	if !hasRoleConflict([]string{"user", "system_admin", "auditor"}, rules) {
		t.Fatal("expected conflicting roles to be rejected")
	}
	if hasRoleConflict([]string{"user", "auditor"}, rules) {
		t.Fatal("expected non-conflicting roles to be accepted")
	}
}
