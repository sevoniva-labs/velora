package identity

import (
	"testing"
	"time"
)

func TestFederatedSessionKeepsSourceWhenMFAIsVerified(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	level, verifiedAt := federatedSessionAttributes(true, now)
	if level != "FEDERATED" || verifiedAt == nil || !verifiedAt.Equal(now) {
		t.Fatalf("level=%q verifiedAt=%v", level, verifiedAt)
	}

	level, verifiedAt = federatedSessionAttributes(false, now)
	if level != "FEDERATED" || verifiedAt != nil {
		t.Fatalf("password-only level=%q verifiedAt=%v", level, verifiedAt)
	}
}
