package authz

import (
	"testing"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
)

func TestAllowedBeforePasswordChange(t *testing.T) {
	allowed := []string{
		forgev1.OperationIdentityServiceGetCurrentUser,
		forgev1.OperationIdentityServiceChangePassword,
		forgev1.OperationIdentityServiceLogout,
	}
	for _, operation := range allowed {
		if !allowedBeforePasswordChange(operation) {
			t.Errorf("operation %s must remain available", operation)
		}
	}
	if allowedBeforePasswordChange(forgev1.OperationPlatformServiceListUsers) {
		t.Fatal("platform operation must be blocked until password change")
	}
}
