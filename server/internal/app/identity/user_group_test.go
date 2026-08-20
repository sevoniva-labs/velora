package identity

import (
	"errors"
	"testing"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func TestNormalizeUserGroup(t *testing.T) {
	group, err := normalizeUserGroup(domain.UserGroup{OrganizationID: " org-1 ", Key: " risk.reviewers ", Name: " Risk Reviewers "}, true)
	if err != nil {
		t.Fatalf("normalize valid group: %v", err)
	}
	if group.Key != "risk.reviewers" || group.Name != "Risk Reviewers" || group.Status != "ACTIVE" {
		t.Fatalf("unexpected normalized group: %+v", group)
	}
	for _, invalid := range []domain.UserGroup{
		{Key: "risk", Name: "Risk", Status: "ACTIVE"},
		{OrganizationID: "org-1", Key: "risk / review", Name: "Risk", Status: "ACTIVE"},
		{OrganizationID: "org-1", Key: "risk", Name: "", Status: "ACTIVE"},
		{OrganizationID: "org-1", Key: "risk", Name: "Risk", Status: "UNKNOWN"},
	} {
		if _, err := normalizeUserGroup(invalid, true); !errors.Is(err, ErrInvalidUserGroup) {
			t.Fatalf("invalid group accepted: %+v, err=%v", invalid, err)
		}
	}
}

func TestNormalizeIDs(t *testing.T) {
	values, err := normalizeIDs([]string{" user-2 ", "user-1", "user-2"}, 3)
	if err != nil {
		t.Fatalf("normalize ids: %v", err)
	}
	if len(values) != 2 || values[0] != "user-2" || values[1] != "user-1" {
		t.Fatalf("unexpected values: %#v", values)
	}
	if _, err := normalizeIDs([]string{"", "user-1"}, 3); err == nil {
		t.Fatal("empty id accepted")
	}
}
