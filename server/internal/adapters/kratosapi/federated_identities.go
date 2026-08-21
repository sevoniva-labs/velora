package kratosapi

import (
	"context"
	"encoding/json"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func (s *PlatformService) ListFederatedIdentityLinks(ctx context.Context, _ *forgev1.ListFederatedIdentityLinksRequest) (*forgev1.ListFederatedIdentityLinksResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	links, err := s.identity.ListFederatedIdentityLinks(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	response := &forgev1.ListFederatedIdentityLinksResponse{Links: make([]*forgev1.FederatedIdentityLink, 0, len(links))}
	for _, link := range links {
		response.Links = append(response.Links, federatedIdentityLinkProto(link))
	}
	return response, nil
}

func (s *PlatformService) LinkFederatedIdentity(ctx context.Context, req *forgev1.LinkFederatedIdentityRequest) (*forgev1.LinkFederatedIdentityResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]string{
		"provider": req.GetProvider(),
		"subject":  req.GetSubject(),
		"user_id":  req.GetUserId(),
	})
	if err != nil {
		return nil, serviceError(err)
	}
	event := newAuditEvent(ctx, principal, "identity_mapping.link", "identity_mapping", req.GetUserId(), map[string]any{
		"provider": req.GetProvider(), "user_id": req.GetUserId(), "approval_id": req.GetApprovalId(),
	})
	var linked domain.FederatedIdentityLink
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "FEDERATED_IDENTITY_LINK",
			Action:      "identity_mapping.link",
			Resource:    "identity_mapping",
			ResourceID:  req.GetUserId(),
			PayloadJSON: string(payload),
		}); executionErr != nil {
			return executionErr
		}
		var linkErr error
		linked, linkErr = s.identity.LinkFederatedIdentity(txCtx, principal, req.GetProvider(), req.GetSubject(), req.GetUserId(), req.GetApprovalId())
		if linkErr == nil {
			event.ResourceID = linked.ID
		}
		return linkErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.LinkFederatedIdentityResponse{Link: federatedIdentityLinkProto(linked)}, nil
}

func (s *PlatformService) UnlinkFederatedIdentity(ctx context.Context, req *forgev1.UnlinkFederatedIdentityRequest) (*forgev1.UnlinkFederatedIdentityResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "identity_mapping.unlink", "identity_mapping", req.GetLinkId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.UnlinkFederatedIdentity(txCtx, principal, req.GetLinkId())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UnlinkFederatedIdentityResponse{}, nil
}

func federatedIdentityLinkProto(link domain.FederatedIdentityLink) *forgev1.FederatedIdentityLink {
	return &forgev1.FederatedIdentityLink{
		Id: link.ID, OrganizationId: link.OrganizationID, Provider: link.Provider, Subject: link.Subject,
		UserId: link.UserID, LoginName: link.LoginName, CreatedBy: link.CreatedBy, ApprovalId: link.ApprovalID,
		CreatedAt: timestamp(link.CreatedAt), LastAuthenticatedAt: optionalTimestamp(link.LastAuthenticatedAt),
	}
}
