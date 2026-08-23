package repository

import (
	"os"
	"strings"
	"testing"
)

func TestIdentityProvisioningUsesDynamicApplicationRoleCatalog(t *testing.T) {
	raw, err := os.ReadFile("identity_provisioning.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if !strings.Contains(source, "FROM portal_application_roles") {
		t.Fatal("application entitlement validation must read the persisted role catalog")
	}
	if strings.Contains(source, `map[string]map[string]struct{}{"spectra"`) {
		t.Fatal("application role validation must not hard-code Spectra")
	}
}
