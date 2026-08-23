package kratosapi

import (
	"context"
	"encoding/json"
	"time"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func (s *PlatformService) CreateTemporaryRoleGrant(ctx context.Context, req *forgev1.CreateTemporaryRoleGrantRequest) (*forgev1.CreateTemporaryRoleGrantResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetValidFrom() == nil || req.GetValidUntil() == nil {
		return nil, serviceError(appidentity.ErrInvalidTemporaryGrant)
	}
	validFrom := req.GetValidFrom().AsTime().UTC()
	validUntil := req.GetValidUntil().AsTime().UTC()
	payloadBytes, err := json.Marshal(map[string]any{
		"reason": req.GetReason(), "role_key": req.GetRoleKey(), "user_id": req.GetUserId(),
		"valid_from": validFrom.Format(time.RFC3339Nano), "valid_until": validUntil.Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, internalError(err)
	}
	var grant domain.TemporaryRoleGrant
	event := newAuditEvent(ctx, principal, "temporary_role_grant.create", "user", req.GetUserId(), map[string]any{"role_key": req.GetRoleKey(), "valid_until": validUntil, "approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "TEMPORARY_ROLE_GRANT", Action: "temporary_role_grant.create", Resource: "user", ResourceID: req.GetUserId(), PayloadJSON: string(payloadBytes),
		}); executionErr != nil {
			return executionErr
		}
		var createErr error
		grant, createErr = s.identity.CreateTemporaryRoleGrant(txCtx, principal, req.GetUserId(), req.GetRoleKey(), req.GetReason(), req.GetApprovalId(), validFrom, validUntil)
		if createErr == nil {
			event.ResourceID = grant.ID
			createErr = s.recomputeApplicationAccess(txCtx, principal)
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateTemporaryRoleGrantResponse{Grant: temporaryRoleGrantProto(grant)}, nil
}

func (s *PlatformService) ListTemporaryRoleGrants(ctx context.Context, _ *forgev1.ListTemporaryRoleGrantsRequest) (*forgev1.ListTemporaryRoleGrantsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := s.identity.ListTemporaryRoleGrants(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	response := &forgev1.ListTemporaryRoleGrantsResponse{Grants: make([]*forgev1.TemporaryRoleGrant, 0, len(grants))}
	for _, grant := range grants {
		response.Grants = append(response.Grants, temporaryRoleGrantProto(grant))
	}
	return response, nil
}

func (s *PlatformService) RevokeTemporaryRoleGrant(ctx context.Context, req *forgev1.RevokeTemporaryRoleGrantRequest) (*forgev1.RevokeTemporaryRoleGrantResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "temporary_role_grant.revoke", "temporary_role_grant", req.GetGrantId(), map[string]any{"reason": req.GetReason()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.identity.RevokeTemporaryRoleGrant(txCtx, principal, req.GetGrantId(), req.GetReason()); err != nil {
			return err
		}
		return s.recomputeApplicationAccess(txCtx, principal)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeTemporaryRoleGrantResponse{}, nil
}

func temporaryRoleGrantProto(grant domain.TemporaryRoleGrant) *forgev1.TemporaryRoleGrant {
	status := "SCHEDULED"
	now := time.Now().UTC()
	if grant.RevokedAt != nil {
		status = "REVOKED"
	} else if !now.Before(grant.ValidUntil) {
		status = "EXPIRED"
	} else if !now.Before(grant.ValidFrom) {
		status = "ACTIVE"
	}
	return &forgev1.TemporaryRoleGrant{
		Id: grant.ID, OrganizationId: grant.OrganizationID, UserId: grant.UserID, RoleKey: grant.RoleKey,
		RequestedBy: grant.RequestedBy, ApprovalId: grant.ApprovalID, Reason: grant.Reason, Status: status,
		ValidFrom: timestamp(grant.ValidFrom), ValidUntil: timestamp(grant.ValidUntil), RevokedAt: optionalTimestamp(grant.RevokedAt),
		RevokedBy: grant.RevokedBy, RevokeReason: grant.RevokeReason, CreatedAt: timestamp(grant.CreatedAt),
	}
}
