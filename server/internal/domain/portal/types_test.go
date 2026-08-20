package portal

import (
	"testing"

	identity "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestCanAccess(t *testing.T) {
	base := Application{OrganizationID: "org-1", Status: StatusEnabled}
	principal := identity.Principal{UserID: "user-1", OrganizationID: "org-1", Roles: []string{"analyst"}}

	tests := []struct {
		name     string
		app      Application
		ctx      AccessContext
		wantPass bool
	}{
		{name: "no policies are public within organization", app: base, ctx: AccessContext{Principal: principal}, wantPass: true},
		{name: "organization is required", app: Application{Status: StatusEnabled}, ctx: AccessContext{Principal: principal}},
		{name: "organization isolation", app: base, ctx: AccessContext{Principal: identity.Principal{OrganizationID: "org-2"}}},
		{name: "disabled application denied", app: Application{OrganizationID: "org-1", Status: StatusDisabled}, ctx: AccessContext{Principal: principal}},
		{name: "everyone policy", app: withPolicy(base, PolicyEveryone, ""), ctx: AccessContext{Principal: principal}, wantPass: true},
		{name: "user policy", app: withPolicy(base, PolicyUser, "user-1"), ctx: AccessContext{Principal: principal}, wantPass: true},
		{name: "role policy", app: withPolicy(base, PolicyRole, "analyst"), ctx: AccessContext{Principal: principal}, wantPass: true},
		{name: "group policy", app: withPolicy(base, PolicyGroup, "finance"), ctx: AccessContext{Principal: principal, Groups: []string{"finance"}}, wantPass: true},
		{name: "organization policy", app: withPolicy(base, PolicyOrganization, "org-1"), ctx: AccessContext{Principal: principal}, wantPass: true},
		{name: "unmatched policy denied", app: withPolicy(base, PolicyRole, "admin"), ctx: AccessContext{Principal: principal}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanAccess(tt.app, tt.ctx); got != tt.wantPass {
				t.Fatalf("CanAccess() = %v, want %v", got, tt.wantPass)
			}
		})
	}
}

func TestValidateApplication(t *testing.T) {
	valid := Application{Code: "crm", Name: "CRM", Status: StatusEnabled, LaunchURL: "https://crm.example.test"}
	if err := ValidateApplication(valid); err != nil {
		t.Fatalf("ValidateApplication(valid) error = %v", err)
	}
	for name, app := range map[string]Application{
		"missing code":        {Name: "CRM", Status: StatusEnabled},
		"missing name":        {Code: "crm", Status: StatusEnabled},
		"bad status":          {Code: "crm", Name: "CRM", Status: "UNKNOWN"},
		"relative url":        {Code: "crm", Name: "CRM", Status: StatusEnabled, LaunchURL: "/crm"},
		"http url":            {Code: "crm", Name: "CRM", Status: StatusEnabled, LaunchURL: "http://crm.example.test"},
		"retired velora oidc": {Code: "legacy", Name: "Legacy", LaunchType: LaunchTypeRetiredVeloraOIDC, Status: StatusEnabled, LaunchURL: "https://legacy.example.test"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateApplication(app); err == nil {
				t.Fatal("ValidateApplication() error = nil, want validation error")
			}
		})
	}
}

func withPolicy(app Application, policyType, value string) Application {
	app.Policies = []AccessPolicy{{Type: policyType, Value: value}}
	return app
}
