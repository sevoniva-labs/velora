package portal

import (
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

func TestApplicationLifecycleAfterUpdate(t *testing.T) {
	tests := []struct {
		name      string
		existing  portaldomain.Application
		input     repository.ApplicationInput
		verified  bool
		lifecycle string
		status    string
		publish   bool
	}{
		{
			name: "published OIDC metadata edit stays enabled",
			existing: portaldomain.Application{LaunchType: "OIDC", LifecycleStatus: portaldomain.LifecyclePublished},
			input: repository.ApplicationInput{LaunchType: "OIDC", Status: portaldomain.StatusEnabled},
			lifecycle: portaldomain.LifecyclePublished, status: portaldomain.StatusEnabled, publish: true,
		},
		{
			name: "verified OIDC recovers from legacy disabled state",
			existing: portaldomain.Application{LaunchType: "OIDC", LifecycleStatus: portaldomain.LifecycleIdentityPending},
			input: repository.ApplicationInput{LaunchType: "OIDC", Status: portaldomain.StatusEnabled}, verified: true,
			lifecycle: portaldomain.LifecyclePublished, status: portaldomain.StatusEnabled, publish: true,
		},
		{
			name: "unverified OIDC cannot bypass onboarding",
			existing: portaldomain.Application{LaunchType: "OIDC", LifecycleStatus: portaldomain.LifecycleIdentityPending},
			input: repository.ApplicationInput{LaunchType: "OIDC", Status: portaldomain.StatusEnabled},
			lifecycle: portaldomain.LifecycleIdentityPending, status: portaldomain.StatusDisabled,
		},
		{
			name: "switching protocol restarts onboarding",
			existing: portaldomain.Application{LaunchType: "URL", LifecycleStatus: portaldomain.LifecyclePublished},
			input: repository.ApplicationInput{LaunchType: "OIDC", Status: portaldomain.StatusEnabled}, verified: true,
			lifecycle: portaldomain.LifecycleIdentityPending, status: portaldomain.StatusDisabled,
		},
		{
			name: "URL application honors explicit disable",
			existing: portaldomain.Application{LaunchType: "URL", LifecycleStatus: portaldomain.LifecyclePublished},
			input: repository.ApplicationInput{LaunchType: "URL", Status: portaldomain.StatusDisabled},
			lifecycle: portaldomain.LifecyclePublished, status: portaldomain.StatusDisabled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lifecycle, status, publish := applicationLifecycleAfterUpdate(tt.existing, tt.input, tt.verified)
			if lifecycle != tt.lifecycle || status != tt.status || publish != tt.publish {
				t.Fatalf("got (%s,%s,%t), want (%s,%s,%t)", lifecycle, status, publish, tt.lifecycle, tt.status, tt.publish)
			}
		})
	}
}
