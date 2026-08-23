package postgres

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationOnboardingProductizationMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("00026_application_onboarding_productization.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(raw)
	for _, required := range []string{
		"portal_application_roles",
		"portal_application_provisioning_targets",
		"portal_application_onboarding_checks",
		"portal_application_onboarding_operations",
		"velora-default-everyone:",
		"ON CONFLICT (organization_id,application_id,role_key) DO NOTHING",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration is missing %q", required)
		}
	}
	if strings.Contains(sql, "secret varchar") || strings.Contains(sql, "secret text") {
		t.Fatal("migration must store a secret reference, never a plaintext secret")
	}
}
