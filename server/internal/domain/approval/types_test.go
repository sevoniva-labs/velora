package approval

import (
	"errors"
	"testing"
	"time"
)

func TestValidateCreationEnforcesMakerCheckerAndMode(t *testing.T) {
	base := Request{OrganizationID: "org", ApplicantID: "maker", RequestType: "ROLE_GRANT", Action: "grant", Resource: "user", Summary: "grant role", RequestDigest: "digest", Mode: ModeAll, RequiredApprovals: 2, ExpiresAt: time.Now().Add(time.Hour)}
	if err := ValidateCreation(base, []string{"checker-a", "checker-b"}); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if err := ValidateCreation(base, []string{"maker", "checker-b"}); !errors.Is(err, ErrMakerChecker) {
		t.Fatalf("maker-checker violation = %v", err)
	}
	base.RequiredApprovals = 1
	if err := ValidateCreation(base, []string{"checker-a", "checker-b"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid ALL threshold = %v", err)
	}
}

func TestResolveStatus(t *testing.T) {
	if got := ResolveStatus(2, 1, 0, 1); got != StatusPending {
		t.Fatalf("pending = %s", got)
	}
	if got := ResolveStatus(2, 2, 0, 0); got != StatusApproved {
		t.Fatalf("approved = %s", got)
	}
	if got := ResolveStatus(2, 1, 1, 0); got != StatusRejected {
		t.Fatalf("rejected = %s", got)
	}
}
