package portal

import (
	"context"
	"strings"
	"testing"
)

func TestDeriveDirectoryTokenIsApplicationBoundAndStable(t *testing.T) {
	secret := strings.Repeat("s", 32)
	first := DeriveDirectoryToken(secret, "app-1")
	if first == "" || !strings.HasPrefix(first, "vd_") {
		t.Fatalf("unexpected directory token: %q", first)
	}
	if first != DeriveDirectoryToken(secret, "app-1") {
		t.Fatal("directory token derivation is not stable")
	}
	if first == DeriveDirectoryToken(secret, "app-2") {
		t.Fatal("directory token was reusable across applications")
	}
	if first == DeriveDirectoryToken(strings.Repeat("x", 32), "app-1") {
		t.Fatal("directory token was reusable after credential rotation")
	}
}

func TestDirectoryUsersRejectsInvalidPageTokenBeforeRepositoryAccess(t *testing.T) {
	service := &Service{}
	_, err := service.DirectoryUsers(context.Background(), DirectoryAccess{ApplicationID: "app-1", OrganizationID: "org-1"}, 100, "not-base64", nil)
	if !strings.Contains(err.Error(), ErrInvalid.Error()) {
		t.Fatalf("invalid page token error = %v", err)
	}
}
