// Package transport adapts the settlement application port to generated
// Kratos HTTP and gRPC contracts.
package transport

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/examples/settlement/application"
	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const PermissionReadSettlement = "settlement:read"

type Service struct {
	forgev1.UnimplementedReferenceSettlementServiceServer
	query *application.QueryService
}

func NewService(query *application.QueryService) (*Service, error) {
	if query == nil {
		return nil, errors.New("settlement query service is required")
	}
	return &Service{query: query}, nil
}

func (s *Service) GetSettlement(ctx context.Context, request *forgev1.GetSettlementRequest) (*forgev1.GetSettlementResponse, error) {
	if request == nil {
		return nil, kratoserrors.BadRequest("INVALID_ARGUMENT", "request is required")
	}
	principal, ok := authn.Principal(ctx)
	if !ok {
		return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	if !principal.HasPermission(PermissionReadSettlement) {
		return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "permission denied")
	}
	if principal.OrganizationID == "" || principal.OrganizationID != request.OrganizationId {
		return nil, kratoserrors.Forbidden("ORGANIZATION_SCOPE_DENIED", "organization scope denied")
	}
	settlement, err := s.query.Get(ctx, request.OrganizationId, request.SettlementId)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidIdentifier):
			return nil, kratoserrors.BadRequest("INVALID_ARGUMENT", "invalid organization or settlement ID")
		case errors.Is(err, domain.ErrNotFound):
			return nil, kratoserrors.NotFound("SETTLEMENT_NOT_FOUND", "settlement not found")
		default:
			return nil, kratoserrors.InternalServer("SETTLEMENT_QUERY_FAILED", "settlement query failed")
		}
	}
	return &forgev1.GetSettlementResponse{Settlement: &forgev1.Settlement{
		Id: settlement.ID, OrganizationId: settlement.OrganizationID, Status: string(settlement.Status),
		Currency: settlement.Currency, AmountMinor: settlement.AmountMinor, Version: settlement.Version,
		UpdatedAt: timestamppb.New(settlement.UpdatedAt),
	}}, nil
}
