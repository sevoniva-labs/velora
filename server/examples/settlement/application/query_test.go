package application

import (
	"context"
	"errors"
	"testing"

	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
)

type readerFunc func(context.Context, string, string) (domain.Settlement, error)

func (f readerFunc) Get(ctx context.Context, organizationID, settlementID string) (domain.Settlement, error) {
	return f(ctx, organizationID, settlementID)
}

func TestQueryServiceReturnsOrganizationScopedSettlement(t *testing.T) {
	service, err := NewQueryService(readerFunc(func(_ context.Context, organizationID, settlementID string) (domain.Settlement, error) {
		return domain.Settlement{ID: settlementID, OrganizationID: organizationID, Status: domain.StatusSettled}, nil
	}))
	if err != nil {
		t.Fatalf("NewQueryService() error = %v", err)
	}
	settlement, err := service.Get(context.Background(), "org-1", "settlement-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if settlement.OrganizationID != "org-1" || settlement.ID != "settlement-1" {
		t.Fatalf("settlement = %#v", settlement)
	}
}

func TestQueryServiceFailsClosedOnRepositoryScopeViolation(t *testing.T) {
	service, err := NewQueryService(readerFunc(func(context.Context, string, string) (domain.Settlement, error) {
		return domain.Settlement{ID: "settlement-1", OrganizationID: "another-org"}, nil
	}))
	if err != nil {
		t.Fatalf("NewQueryService() error = %v", err)
	}
	if _, err := service.Get(context.Background(), "org-1", "settlement-1"); !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("Get() error = %v, want ErrScopeViolation", err)
	}
}

func TestQueryServiceRejectsUnsafeIdentifiersBeforeRepositoryCall(t *testing.T) {
	called := false
	service, err := NewQueryService(readerFunc(func(context.Context, string, string) (domain.Settlement, error) {
		called = true
		return domain.Settlement{}, nil
	}))
	if err != nil {
		t.Fatalf("NewQueryService() error = %v", err)
	}
	if _, err := service.Get(context.Background(), "org-1", "../settlement"); !errors.Is(err, ErrInvalidIdentifier) {
		t.Fatalf("Get() error = %v, want ErrInvalidIdentifier", err)
	}
	if called {
		t.Fatal("repository called for an invalid identifier")
	}
}
