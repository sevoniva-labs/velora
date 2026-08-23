package kratosapi

import (
	"testing"

	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

func TestOnboardingStateFailsClosedWithoutAccessPolicy(t *testing.T) {
	app := portaldomain.Application{LaunchType: "OIDC", Status: portaldomain.StatusDisabled, LifecycleStatus: portaldomain.LifecycleReady}
	binding := portaldomain.IdentityBinding{ID: "binding-1", VerificationStatus: portaldomain.VerificationPassed}
	target := portaldomain.ProvisioningTarget{ID: "target-1", DeliveryStatus: "HEALTHY"}
	status, _, blockers, canPublish := onboardingState(app, binding, target, nil)
	if status != "DRAFT" || canPublish || len(blockers) != 1 {
		t.Fatalf("state = %q, blockers=%v, canPublish=%v", status, blockers, canPublish)
	}
}

func TestOnboardingStateRequiresHealthyGenericProvisioningTarget(t *testing.T) {
	app := portaldomain.Application{Code: "order-center", LaunchType: "OIDC", Status: portaldomain.StatusDisabled, LifecycleStatus: portaldomain.LifecycleReady, Policies: []portaldomain.AccessPolicy{{Type: portaldomain.PolicyEveryone}}}
	binding := portaldomain.IdentityBinding{ID: "binding-1", VerificationStatus: portaldomain.VerificationPassed}
	status, _, blockers, canPublish := onboardingState(app, binding, portaldomain.ProvisioningTarget{ID: "target-1", DeliveryStatus: "PENDING"}, nil)
	if status != "ACTION_REQUIRED" || canPublish || len(blockers) != 1 {
		t.Fatalf("state = %q, blockers=%v, canPublish=%v", status, blockers, canPublish)
	}
	checks := []portaldomain.OnboardingCheck{{CheckType: "access_policy", Result: "PASSED"}, {CheckType: "oidc_discovery", Result: "PASSED"}, {CheckType: "provisioning_challenge", Result: "PASSED"}, {CheckType: "provisioning_duplicate", Result: "PASSED"}, {CheckType: "provisioning_stale", Result: "PASSED"}}
	status, _, blockers, canPublish = onboardingState(app, binding, portaldomain.ProvisioningTarget{ID: "target-1", DeliveryStatus: "HEALTHY"}, checks)
	if status != "VERIFIED" || !canPublish || len(blockers) != 0 {
		t.Fatalf("state = %q, blockers=%v, canPublish=%v", status, blockers, canPublish)
	}
}
