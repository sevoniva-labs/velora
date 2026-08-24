package configchange

import (
	"context"
	"testing"

	identitydomain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	platformconfig "github.com/sevoniva-labs/velora/server/internal/platform/configchange"
)

type versioningRepository struct {
	latest  uint64
	created platformconfig.Change
}

func (r *versioningRepository) List(context.Context, string) ([]platformconfig.Change, error) {
	return nil, nil
}

func (r *versioningRepository) ByID(context.Context, string, string) (platformconfig.Change, error) {
	return platformconfig.Change{}, nil
}

func (r *versioningRepository) LatestVersion(context.Context, string, string, string, string) (uint64, error) {
	return r.latest, nil
}

func (r *versioningRepository) Create(_ context.Context, change platformconfig.Change) (platformconfig.Change, error) {
	r.created = change
	return change, nil
}

func (r *versioningRepository) Update(_ context.Context, change platformconfig.Change) (platformconfig.Change, error) {
	return change, nil
}

func TestCreateAssignsNextVersionOnServer(t *testing.T) {
	repo := &versioningRepository{latest: 7}
	service := NewService(repo)
	created, err := service.Create(context.Background(), identitydomain.Principal{Type: "USER", UserID: "user-1", OrganizationID: "org-1"}, CreateInput{
		Namespace: " production ", Group: " portal ", DataID: " theme ", Version: 99, ExpectedPreviousVersion: 98,
		ValueDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ValueRef: "cos://velora/config/theme", Sensitive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 8 || created.ExpectedPreviousVersion != 7 {
		t.Fatalf("versions = %d/%d, want 8/7", created.Version, created.ExpectedPreviousVersion)
	}
	if created.Namespace != "production" || created.Group != "portal" || created.DataID != "theme" {
		t.Fatalf("config identity was not normalized: %#v", created)
	}
}
