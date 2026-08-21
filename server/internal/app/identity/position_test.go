package identity

import (
	"errors"
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestNormalizePosition(t *testing.T) {
	item, err := normalizePosition(domain.Position{OrganizationID: " org-1 ", DepartmentID: " dept-1 ", Key: " checker ", Name: " Checker ", SortOrder: 10}, true)
	if err != nil {
		t.Fatalf("normalize valid position: %v", err)
	}
	if item.DepartmentID != "dept-1" || item.Key != "checker" || item.Name != "Checker" || item.Status != "ACTIVE" {
		t.Fatalf("unexpected normalized position: %+v", item)
	}
	for _, invalid := range []domain.Position{
		{OrganizationID: "", DepartmentID: "dept-1", Key: "checker", Name: "Checker", Status: "ACTIVE"},
		{OrganizationID: "org-1", DepartmentID: "", Key: "checker", Name: "Checker", Status: "ACTIVE"},
		{OrganizationID: "org-1", DepartmentID: "dept-1", Key: "checker / maker", Name: "Checker", Status: "ACTIVE"},
		{OrganizationID: "org-1", DepartmentID: "dept-1", Key: "checker", Name: "", Status: "ACTIVE"},
		{OrganizationID: "org-1", DepartmentID: "dept-1", Key: "checker", Name: "Checker", Status: "UNKNOWN"},
	} {
		if _, err := normalizePosition(invalid, true); !errors.Is(err, ErrInvalidPosition) {
			t.Fatalf("invalid position accepted: %+v, err=%v", invalid, err)
		}
	}
}

func TestValidatePositionDepartment(t *testing.T) {
	active := []domain.Department{{ID: "dept-1", OrganizationID: "org-1", Status: "ACTIVE"}}
	if err := validatePositionDepartment(active, domain.Position{OrganizationID: "org-1", DepartmentID: "dept-1", Status: "ACTIVE"}); err != nil {
		t.Fatalf("active department rejected: %v", err)
	}
	disabled := []domain.Department{{ID: "dept-1", OrganizationID: "org-1", Status: "DISABLED"}}
	if err := validatePositionDepartment(disabled, domain.Position{OrganizationID: "org-1", DepartmentID: "dept-1", Status: "ACTIVE"}); !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("active position under disabled department accepted: %v", err)
	}
	if err := validatePositionDepartment(active, domain.Position{OrganizationID: "other", DepartmentID: "dept-1", Status: "ACTIVE"}); !errors.Is(err, ErrInvalidPosition) {
		t.Fatalf("cross-organization department accepted: %v", err)
	}
}
