package identity

import "testing"

func TestEffectiveDataScopeAllows(t *testing.T) {
	tests := []struct {
		name         string
		scope        EffectiveDataScope
		userID       string
		departmentID string
		actorUserID  string
		want         bool
	}{
		{name: "organization", scope: EffectiveDataScope{OrganizationWide: true}, userID: "other", actorUserID: "actor", want: true},
		{name: "self", scope: EffectiveDataScope{Self: true}, userID: "actor", actorUserID: "actor", want: true},
		{name: "self rejects other", scope: EffectiveDataScope{Self: true}, userID: "other", actorUserID: "actor", want: false},
		{name: "department", scope: EffectiveDataScope{DepartmentIDs: []string{"dept-a"}}, userID: "other", departmentID: "dept-a", actorUserID: "actor", want: true},
		{name: "department rejects other", scope: EffectiveDataScope{DepartmentIDs: []string{"dept-a"}}, userID: "other", departmentID: "dept-b", actorUserID: "actor", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.Allows(tt.userID, tt.departmentID, tt.actorUserID); got != tt.want {
				t.Fatalf("Allows() = %v, want %v", got, tt.want)
			}
		})
	}
}
