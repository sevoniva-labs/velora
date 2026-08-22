package repository

import (
	"os"
	"strings"
	"testing"
)

// Keep the provisioning projection aligned with the canonical portal schema.
// This catches column-name drift without requiring an external test database.
func TestIdentityProvisioningUsesPortalApplicationCodeColumn(t *testing.T) {
	raw, err := os.ReadFile("identity_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if strings.Contains(source, "portal_applications WHERE organization_id=? AND application_code=?") ||
		strings.Contains(source, "SELECT a.application_code") {
		t.Fatal("provisioning queries must use portal_applications.code")
	}
	if !strings.Contains(source, "portal_applications WHERE organization_id=? AND code=?") ||
		!strings.Contains(source, "SELECT a.code,a.name") {
		t.Fatal("provisioning query is missing the canonical portal application code column")
	}
}
