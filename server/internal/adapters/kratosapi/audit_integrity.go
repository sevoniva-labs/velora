package kratosapi

import (
	"context"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
)

func (s *PlatformService) VerifyAuditIntegrity(ctx context.Context, _ *forgev1.VerifyAuditIntegrityRequest) (*forgev1.VerifyAuditIntegrityResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.audit.VerifyIntegrity(ctx, principal.OrganizationID); err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.VerifyAuditIntegrityResponse{Verified: true}, nil
}
