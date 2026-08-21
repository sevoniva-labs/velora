package identity

import (
	"errors"
	"testing"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestNormalizeUserAssignments(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	until := now.Add(24 * time.Hour)
	items, err := normalizeUserAssignments("org-1", "user-1", []domain.UserAssignment{
		{DepartmentID: " dept-1 ", PositionID: " position-1 ", Primary: true, ValidUntil: &until},
		{DepartmentID: "dept-2", ValidFrom: now.Add(time.Hour)},
	}, now)
	if err != nil {
		t.Fatalf("normalize valid assignments: %v", err)
	}
	if len(items) != 2 || items[0].OrganizationID != "org-1" || items[0].UserID != "user-1" || items[0].ValidFrom != now {
		t.Fatalf("unexpected assignments: %+v", items)
	}
	invalidUntil := now.Add(-time.Minute)
	for _, invalid := range [][]domain.UserAssignment{
		{{DepartmentID: "dept-1"}},
		{{DepartmentID: "dept-1", Primary: true}, {DepartmentID: "dept-2", Primary: true}},
		{{DepartmentID: "dept-1", PositionID: "position-1", Primary: true}, {DepartmentID: "dept-1", PositionID: "position-1"}},
		{{DepartmentID: "dept-1", Primary: true, ValidFrom: now, ValidUntil: &invalidUntil}},
	} {
		if _, err := normalizeUserAssignments("org-1", "user-1", invalid, now); !errors.Is(err, ErrInvalidUserAssignment) {
			t.Fatalf("invalid assignments accepted: %+v, err=%v", invalid, err)
		}
	}
}

func TestValidateAssignmentTargets(t *testing.T) {
	departments := []domain.Department{{ID: "dept-1", OrganizationID: "org-1", Status: "ACTIVE"}}
	positions := []domain.Position{{ID: "position-1", OrganizationID: "org-1", DepartmentID: "dept-1", Status: "ACTIVE"}}
	valid := []domain.UserAssignment{{OrganizationID: "org-1", DepartmentID: "dept-1", PositionID: "position-1"}}
	if err := validateAssignmentTargets(departments, positions, valid); err != nil {
		t.Fatalf("valid targets rejected: %v", err)
	}
	invalid := []domain.UserAssignment{{OrganizationID: "org-1", DepartmentID: "dept-1", PositionID: "missing"}}
	if err := validateAssignmentTargets(departments, positions, invalid); !errors.Is(err, ErrInvalidUserAssignment) {
		t.Fatalf("missing position accepted: %v", err)
	}
}
