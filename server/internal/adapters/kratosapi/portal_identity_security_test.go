package kratosapi

import (
	"context"
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestSafeIdentityAdminURLRejectsMalformedURLWithoutPanic(t *testing.T) {
	for _, raw := range []string{"%", "http://[::1", "://bad"} {
		if safeIdentityAdminURL(raw, []string{"identity.example.test"}) {
			t.Fatalf("malformed admin URL %q was accepted", raw)
		}
	}
}

func TestSafeIdentityAdminURLRequiresAllowedHTTPSHost(t *testing.T) {
	if !safeIdentityAdminURL("https://identity.example.test/admin", []string{"identity.example.test"}) {
		t.Fatal("allowlisted HTTPS admin URL was rejected")
	}
	for _, raw := range []string{
		"http://identity.example.test/admin",
		"https://attacker.example.test/admin",
		"https://identity.example.test/admin?next=https://attacker.example.test",
		"https://user:pass@identity.example.test/admin",
	} {
		if safeIdentityAdminURL(raw, []string{"identity.example.test"}) {
			t.Fatalf("unsafe admin URL %q was accepted", raw)
		}
	}
}

func TestCasdoorAutomationFailsClosedWithoutApprovalService(t *testing.T) {
	service := &PortalService{}
	if err := service.authorizeCasdoorAutomation(context.Background(), domain.Principal{Type: "USER", UserID: "u", OrganizationID: "o"}, "approval-1", "UPSERT", "app-1", map[string]any{"client_id": "public"}); err == nil {
		t.Fatal("automation was authorized without approval service")
	}
}
