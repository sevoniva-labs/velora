package repository

import (
	"testing"
	"time"

	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

func TestResolveProfileAccessRejectsInactiveUsers(t *testing.T) {
	grants := []portaldomain.AccessGrant{{ID: "grant-all", SubjectType: portaldomain.AccessSubjectEveryone, Effect: portaldomain.AccessEffectAllow, Status: portaldomain.StatusActive}}
	profile := accessProfile{AccessSubjectProfile: portaldomain.AccessSubjectProfile{UserID: "user-1", LoginName: "disabled"}, Status: portaldomain.StatusDisabled}
	got := resolveProfileAccess(grants, profile, nil, time.Now().UTC())
	if got.Allowed || len(got.SourceGrantIDs) != 0 {
		t.Fatalf("inactive user retained effective access: %#v", got)
	}
}

func TestResolveProfileAccessAllowsActiveUsers(t *testing.T) {
	grants := []portaldomain.AccessGrant{{ID: "grant-all", SubjectType: portaldomain.AccessSubjectEveryone, Effect: portaldomain.AccessEffectAllow, Status: portaldomain.StatusActive}}
	profile := accessProfile{AccessSubjectProfile: portaldomain.AccessSubjectProfile{UserID: "user-1", LoginName: "active"}, Status: portaldomain.StatusActive}
	got := resolveProfileAccess(grants, profile, nil, time.Now().UTC())
	if !got.Allowed || len(got.SourceGrantIDs) != 1 || got.SourceGrantIDs[0] != "grant-all" {
		t.Fatalf("active user did not receive effective access: %#v", got)
	}
}
