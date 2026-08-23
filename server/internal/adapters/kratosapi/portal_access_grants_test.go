package kratosapi

import (
	"context"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	identitydomain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestLegacyPortalPolicyMutationIsRetired(t *testing.T) {
	service := &PortalService{}
	ctx := authn.WithPrincipal(context.Background(), identitydomain.Principal{Type: "USER", UserID: "user-1", OrganizationID: "org-1"})
	_, err := service.ReplacePortalApplicationPolicies(ctx, &forgev1.ReplacePortalApplicationPoliciesRequest{ApplicationId: "app-1"})
	if err == nil {
		t.Fatal("legacy policy mutation was accepted")
	}
	if reason := kratoserrors.FromError(err).Reason; reason != "LEGACY_POLICY_MUTATION_DISABLED" {
		t.Fatalf("unexpected error reason %q", reason)
	}
}

func TestAccessGrantsApprovalPayloadIsStableAndMinimal(t *testing.T) {
	validUntil := time.Date(2026, 8, 25, 8, 30, 0, 0, time.UTC)
	payload := accessGrantsApprovalPayload([]*forgev1.PortalApplicationAccessGrant{{
		Id: "grant-1", ApplicationId: "app-1", SubjectType: "DEPARTMENT", SubjectId: "dept-1",
		IncludeDescendants: true, Effect: "ALLOW", Roles: []string{"viewer"}, Status: "ACTIVE",
		Reason: "业务访问", Version: 2, ValidUntil: timestamppb.New(validUntil),
	}})
	if len(payload) != 1 {
		t.Fatalf("expected one grant, got %d", len(payload))
	}
	grant := payload[0]
	if grant["application_id"] != "app-1" || grant["subject_type"] != "DEPARTMENT" || grant["valid_until"] != validUntil.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected approval payload: %#v", grant)
	}
	if _, exists := grant["valid_from"]; exists {
		t.Fatal("unset optional timestamps must not alter the approval digest")
	}
}
