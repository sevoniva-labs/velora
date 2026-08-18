package application

import (
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

func TestCanAccess(t *testing.T) {
	user := &auth.CurrentUser{
		ID:           "u-1",
		Username:     "carson",
		Organization: "sevoniva",
		Roles:        []string{"developer", "velora_admin"},
		Groups:       []string{"platform-team"},
	}

	tests := []struct {
		name     string
		policies []AccessPolicy
		want     bool
	}{
		{"no policies -> everyone", nil, true},
		{"everyone", []AccessPolicy{{PolicyType: PolicyTypeEveryone}}, true},
		{"org match", []AccessPolicy{{PolicyType: PolicyTypeOrganization, Value: "sevoniva"}}, true},
		{"org mismatch", []AccessPolicy{{PolicyType: PolicyTypeOrganization, Value: "other"}}, false},
		{"role match", []AccessPolicy{{PolicyType: PolicyTypeRole, Value: "velora_admin"}}, true},
		{"role mismatch", []AccessPolicy{{PolicyType: PolicyTypeRole, Value: "auditor"}}, false},
		{"group match", []AccessPolicy{{PolicyType: PolicyTypeGroup, Value: "platform-team"}}, true},
		{"group mismatch", []AccessPolicy{{PolicyType: PolicyTypeGroup, Value: "security-team"}}, false},
		{"user match", []AccessPolicy{{PolicyType: PolicyTypeUser, Value: "u-1"}}, true},
		{"user mismatch", []AccessPolicy{{PolicyType: PolicyTypeUser, Value: "u-2"}}, false},
		{
			"mixed deny", []AccessPolicy{
				{PolicyType: PolicyTypeOrganization, Value: "other"},
				{PolicyType: PolicyTypeRole, Value: "auditor"},
			}, false,
		},
		{
			"mixed allow", []AccessPolicy{
				{PolicyType: PolicyTypeOrganization, Value: "other"},
				{PolicyType: PolicyTypeRole, Value: "developer"},
			}, true,
		},
		{"nil user denied", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := user
			if tt.policies == nil && tt.name == "nil user denied" {
				u = nil
			}
			if got := CanAccess(u, tt.policies); got != tt.want {
				t.Errorf("CanAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 回归：EVERYONE 策略应放行任何登录用户（含空角色）。
func TestCanAccessEveryoneAnyUser(t *testing.T) {
	u := &auth.CurrentUser{ID: "x", Roles: nil, Groups: nil, Organization: ""}
	if !CanAccess(u, []AccessPolicy{{PolicyType: PolicyTypeEveryone}}) {
		t.Fatal("EVERYONE 策略应放行任意登录用户")
	}
}

// 回归：用户同时命中任一策略即放行（OR 语义）。
func TestCanAccessOrSemantics(t *testing.T) {
	u := &auth.CurrentUser{ID: "u-9", Organization: "sevoniva"}
	policies := []AccessPolicy{
		{PolicyType: PolicyTypeRole, Value: "admin"},
		{PolicyType: PolicyTypeOrganization, Value: "sevoniva"},
	}
	if !CanAccess(u, policies) {
		t.Fatal("OR 语义：命中 ORGANIZATION 即应放行")
	}
}
