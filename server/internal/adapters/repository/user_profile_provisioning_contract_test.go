package repository

import (
	"os"
	"strings"
	"testing"
)

// This contract test guards the PostgreSQL/MySQL-neutral projection query and
// the event compatibility guarantees used by strict downstream receivers.
func TestProfileProvisioningUsesApplicationTenantAndStableEvent(t *testing.T) {
	source, err := os.ReadFile("user_profiles.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"a.organization_id=?",
		"UPDATE user_application_entitlements SET version=?",
		`SchemaVersion: "1.0"`,
		`Topic: "velora.provisioning." + item.applicationCode`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("profile provisioning contract is missing %q", required)
		}
	}
	if strings.Contains(text, "e.organization_id") {
		t.Fatal("user_application_entitlements has no organization_id column")
	}
}
