package portal

import (
	"reflect"
	"testing"
	"time"
)

func TestResolveAccessGrantsCombinesRolesAndExplainsSources(t *testing.T) {
	now := time.Now().UTC()
	profile := AccessSubjectProfile{UserID: "user-1", Roles: []string{"developer"}, GroupIDs: []string{"group-1"}, DepartmentIDs: []string{"department-child"}}
	grants := []AccessGrant{
		{ID: "department", SubjectType: AccessSubjectDepartment, SubjectID: "department-root", IncludeDescendants: true, Effect: AccessEffectAllow, Status: StatusActive, Roles: []string{"reader"}},
		{ID: "role", SubjectType: AccessSubjectPlatformRole, SubjectID: "developer", Effect: AccessEffectAllow, Status: StatusActive, Roles: []string{"editor", "reader"}},
	}
	got := ResolveAccessGrants(grants, profile, map[string]string{"department-child": "department-root"}, now)
	if !got.Allowed || !reflect.DeepEqual(got.Roles, []string{"editor", "reader"}) || !reflect.DeepEqual(got.SourceGrantIDs, []string{"department", "role"}) {
		t.Fatalf("ResolveAccessGrants() = %#v", got)
	}
}

func TestResolveAccessGrantsExclusionWins(t *testing.T) {
	profile := AccessSubjectProfile{UserID: "user-1"}
	grants := []AccessGrant{
		{ID: "everyone", SubjectType: AccessSubjectEveryone, Effect: AccessEffectAllow, Status: StatusActive},
		{ID: "exclude", SubjectType: AccessSubjectUser, SubjectID: "user-1", Effect: AccessEffectExclude, Status: StatusActive},
	}
	if got := ResolveAccessGrants(grants, profile, nil, time.Now().UTC()); got.Allowed || len(got.Roles) != 0 || len(got.SourceGrantIDs) != 0 {
		t.Fatalf("exclusion did not win: %#v", got)
	}
}

func TestResolveAccessGrantsRejectsExpiredAndDisabledRules(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	grants := []AccessGrant{
		{ID: "expired", SubjectType: AccessSubjectEveryone, Effect: AccessEffectAllow, Status: StatusActive, ValidUntil: &past},
		{ID: "disabled", SubjectType: AccessSubjectEveryone, Effect: AccessEffectAllow, Status: StatusDisabled},
	}
	if got := ResolveAccessGrants(grants, AccessSubjectProfile{UserID: "user-1"}, nil, now); got.Allowed {
		t.Fatalf("inactive grants allowed access: %#v", got)
	}
}
