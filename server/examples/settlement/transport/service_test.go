package transport

import (
	"context"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/examples/settlement/application"
	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
	"github.com/sevoniva-labs/velora/server/examples/settlement/repository"
	identitydomain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	reader, err := repository.NewMemory(domain.Settlement{
		ID: "settlement-1", OrganizationID: "org-1", Status: domain.StatusSettled, Currency: "CNY", AmountMinor: 100,
	})
	if err != nil {
		t.Fatalf("NewMemory() error = %v", err)
	}
	query, err := application.NewQueryService(reader)
	if err != nil {
		t.Fatalf("NewQueryService() error = %v", err)
	}
	service, err := NewService(query)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func TestGetSettlementEnforcesPermissionAndOrganization(t *testing.T) {
	service := newTestService(t)
	request := &forgev1.GetSettlementRequest{OrganizationId: "org-1", SettlementId: "settlement-1"}
	tests := []struct {
		name      string
		principal *identitydomain.Principal
		wantCode  int
	}{
		{name: "unauthenticated", wantCode: 401},
		{name: "permission denied", principal: &identitydomain.Principal{Type: "TOKEN", OrganizationID: "org-1"}, wantCode: 403},
		{name: "organization denied", principal: &identitydomain.Principal{
			Type: "TOKEN", OrganizationID: "org-2", Permissions: []string{PermissionReadSettlement}, Scopes: []string{PermissionReadSettlement},
		}, wantCode: 403},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			if test.principal != nil {
				ctx = authn.WithPrincipal(ctx, *test.principal)
			}
			_, err := service.GetSettlement(ctx, request)
			if err == nil || kratoserrors.Code(err) != test.wantCode {
				t.Fatalf("GetSettlement() error = %v, code = %d", err, kratoserrors.Code(err))
			}
		})
	}
}

func TestGetSettlementReturnsAuthorizedProjection(t *testing.T) {
	service := newTestService(t)
	ctx := authn.WithPrincipal(context.Background(), identitydomain.Principal{
		Type: "TOKEN", OrganizationID: "org-1",
		Permissions: []string{PermissionReadSettlement}, Scopes: []string{PermissionReadSettlement},
	})
	response, err := service.GetSettlement(ctx, &forgev1.GetSettlementRequest{OrganizationId: "org-1", SettlementId: "settlement-1"})
	if err != nil {
		t.Fatalf("GetSettlement() error = %v", err)
	}
	if response.GetSettlement().GetAmountMinor() != 100 || response.GetSettlement().GetCurrency() != "CNY" {
		t.Fatalf("response = %#v", response)
	}
}
