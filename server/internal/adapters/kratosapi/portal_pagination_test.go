package kratosapi

import (
	"testing"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
)

func TestAdminApplicationPageSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  *forgev1.ListAdminPortalApplicationsRequest
		want int
	}{
		{name: "default remains unset", req: &forgev1.ListAdminPortalApplicationsRequest{}, want: 0},
		{name: "legacy client limit", req: &forgev1.ListAdminPortalApplicationsRequest{Limit: 50}, want: 50},
		{name: "page size wins", req: &forgev1.ListAdminPortalApplicationsRequest{Limit: 50, PageSize: 20}, want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := adminApplicationPageSize(tt.req); got != tt.want {
				t.Fatalf("adminApplicationPageSize() = %d, want %d", got, tt.want)
			}
		})
	}
}
