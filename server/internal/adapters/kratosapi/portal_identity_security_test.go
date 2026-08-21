package kratosapi

import "testing"

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
