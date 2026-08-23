package portal

import "testing"

func TestValidApplicationRoleKey(t *testing.T) {
	for _, value := range []string{"developer", "project_admin", "ci-service", "role2"} {
		if !validApplicationRoleKey(value) {
			t.Fatalf("validApplicationRoleKey(%q) = false", value)
		}
	}
	for _, value := range []string{"", "Admin", "role space", "角色", "role/owner"} {
		if validApplicationRoleKey(value) {
			t.Fatalf("validApplicationRoleKey(%q) = true", value)
		}
	}
}
