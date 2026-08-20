package identity

import (
	"errors"
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestNormalizeDepartment(t *testing.T) {
	item, err := normalizeDepartment(domain.Department{OrganizationID: " org-1 ", Key: " risk.ops ", Name: " Risk Operations ", SortOrder: 10}, true)
	if err != nil {
		t.Fatalf("normalize valid department: %v", err)
	}
	if item.OrganizationID != "org-1" || item.Key != "risk.ops" || item.Name != "Risk Operations" || item.Status != "ACTIVE" {
		t.Fatalf("unexpected normalized department: %+v", item)
	}
	for _, invalid := range []domain.Department{
		{OrganizationID: "", Key: "risk", Name: "Risk", Status: "ACTIVE"},
		{OrganizationID: "org-1", Key: "risk / ops", Name: "Risk", Status: "ACTIVE"},
		{OrganizationID: "org-1", Key: "risk", Name: "", Status: "ACTIVE"},
		{OrganizationID: "org-1", Key: "risk", Name: "Risk", Status: "UNKNOWN"},
		{OrganizationID: "org-1", Key: "risk", Name: "Risk", Status: "ACTIVE", SortOrder: -1},
	} {
		if _, err := normalizeDepartment(invalid, true); !errors.Is(err, ErrInvalidDepartment) {
			t.Fatalf("invalid department accepted: %+v, err=%v", invalid, err)
		}
	}
}

func TestValidateDepartmentHierarchy(t *testing.T) {
	items := []domain.Department{
		{ID: "root", OrganizationID: "org-1", Key: "root", Status: "ACTIVE"},
		{ID: "child", OrganizationID: "org-1", ParentID: "root", Key: "child", Status: "ACTIVE"},
	}
	if err := validateDepartmentHierarchy(items, domain.Department{ID: "child", OrganizationID: "org-1", ParentID: "", Status: "ACTIVE"}, false); err != nil {
		t.Fatalf("valid reparent rejected: %v", err)
	}
	if err := validateDepartmentHierarchy(items, domain.Department{ID: "root", OrganizationID: "org-1", ParentID: "child", Status: "ACTIVE"}, false); !errors.Is(err, ErrInvalidDepartment) {
		t.Fatalf("cycle accepted: %v", err)
	}
	if err := validateDepartmentHierarchy(items, domain.Department{ID: "root", OrganizationID: "org-1", Status: "DISABLED"}, false); !errors.Is(err, ErrInvalidDepartment) {
		t.Fatalf("active child under disabled parent accepted: %v", err)
	}
	disabled := []domain.Department{{ID: "root", OrganizationID: "org-1", Status: "DISABLED"}}
	if err := validateDepartmentHierarchy(disabled, domain.Department{OrganizationID: "org-1", ParentID: "root", Status: "ACTIVE"}, true); !errors.Is(err, ErrInvalidDepartment) {
		t.Fatalf("active department under disabled parent accepted: %v", err)
	}
}
