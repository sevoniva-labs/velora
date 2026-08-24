package kratosapi

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestPlatformProtoMappings(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	locked := now.Add(time.Minute)
	user := userProto(domain.User{
		ID: "user-1", OrganizationID: "org-1", LoginName: "alice", Status: "ACTIVE",
		LockedUntil: &locked, CreatedAt: now, Roles: []string{"auditor"},
	})
	if user.Id != "user-1" || user.OrganizationId != "org-1" || user.LockedUntil.AsTime() != locked {
		t.Fatalf("unexpected user mapping: %+v", user)
	}
	department := departmentProto(domain.Department{ID: "dept-1", OrganizationID: "org-1", ParentID: "dept-0", Key: "risk", Name: "Risk", Status: "ACTIVE", SortOrder: 10, CreatedAt: now})
	if department.Id != "dept-1" || department.ParentId != "dept-0" || department.DepartmentKey != "risk" || department.SortOrder != 10 {
		t.Fatalf("unexpected department mapping: %+v", department)
	}
	position := positionProto(domain.Position{ID: "position-1", OrganizationID: "org-1", DepartmentID: "dept-1", Key: "reviewer", Name: "Reviewer", Status: "ACTIVE", SortOrder: 20, CreatedAt: now})
	if position.Id != "position-1" || position.DepartmentId != "dept-1" || position.PositionKey != "reviewer" || position.SortOrder != 20 {
		t.Fatalf("unexpected position mapping: %+v", position)
	}
	group := userGroupProto(domain.UserGroup{ID: "group-1", OrganizationID: "org-1", Key: "reviewers", Name: "Reviewers", Status: "ACTIVE", Roles: []string{"auditor"}, MemberIDs: []string{"user-1"}, MemberCount: 1, CreatedAt: now})
	if group.Id != "group-1" || group.GroupKey != "reviewers" || group.MemberCount != 1 || len(group.Roles) != 1 {
		t.Fatalf("unexpected user group mapping: %+v", group)
	}
	assignment := userAssignmentProto(domain.UserAssignment{ID: "assignment-1", OrganizationID: "org-1", UserID: "user-1", DepartmentID: "dept-1", PositionID: "position-1", Primary: true, ValidFrom: now, CreatedAt: now})
	if assignment.Id != "assignment-1" || assignment.DepartmentId != "dept-1" || assignment.PositionId != "position-1" || !assignment.Primary {
		t.Fatalf("unexpected user assignment mapping: %+v", assignment)
	}
	grant := temporaryRoleGrantProto(domain.TemporaryRoleGrant{ID: "grant-1", OrganizationID: "org-1", UserID: "user-1", LoginName: "alice", DisplayName: "Alice", RoleKey: "auditor", ValidFrom: now, ValidUntil: now.Add(time.Hour), CreatedAt: now})
	if grant.UserId != "user-1" || grant.LoginName != "alice" || grant.DisplayName != "Alice" {
		t.Fatalf("unexpected temporary grant user mapping: %+v", grant)
	}
	policy := securityPolicyProto(domain.SecurityPolicy{PasswordMinLength: 14, SessionTTLSeconds: 3600, MaxConcurrentSessions: 2})
	if policy.PasswordMinLength != 14 || policy.SessionTtlSeconds != 3600 || policy.MaxActiveSessions != 2 {
		t.Fatalf("unexpected policy mapping: %+v", policy)
	}
	event := auditEventProto(audit.Event{ID: "event-1", OccurredAt: now, Details: map[string]any{"safe": true}})
	if event.Id != "event-1" || event.DetailsJson != `{"safe":true}` {
		t.Fatalf("unexpected audit mapping: %+v", event)
	}
}

func TestPlatformNumericMappingsRejectInvalidValues(t *testing.T) {
	if _, err := checkedInt(-1); err == nil {
		t.Fatal("negative quota was accepted")
	}
	if strconv.IntSize == 32 {
		if _, err := checkedInt(int64(math.MaxInt32) + 1); err == nil {
			t.Fatal("overflowing 32-bit quota was accepted")
		}
	}
	if _, err := securityPolicyDomain(nil); err == nil {
		t.Fatal("missing security policy was accepted")
	}
}
