package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
)

func TestMemoryRepositoryKeepsOrganizationsIsolated(t *testing.T) {
	repository, err := NewMemory(
		domain.Settlement{ID: "same-id", OrganizationID: "org-1", AmountMinor: 100},
		domain.Settlement{ID: "same-id", OrganizationID: "org-2", AmountMinor: 200},
	)
	if err != nil {
		t.Fatalf("NewMemory() error = %v", err)
	}
	item, err := repository.Get(context.Background(), "org-2", "same-id")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if item.AmountMinor != 200 {
		t.Fatalf("AmountMinor = %d, want 200", item.AmountMinor)
	}
	if _, err := repository.Get(context.Background(), "org-3", "same-id"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}
