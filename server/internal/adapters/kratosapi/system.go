package kratosapi

import (
	"context"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/health"
)

type SystemService struct {
	forgev1.UnimplementedSystemServiceServer
	cfg       config.Config
	version   string
	checks    []health.Check
	providers map[string]string
}

func NewSystemService(cfg config.Config, version string, checks []health.Check, providers map[string]string) *SystemService {
	return &SystemService{cfg: cfg, version: version, checks: checks, providers: providers}
}

func (s *SystemService) Health(context.Context, *forgev1.HealthRequest) (*forgev1.HealthResponse, error) {
	authMode := strings.ToLower(strings.TrimSpace(s.cfg.Security.AuthMode))
	return &forgev1.HealthResponse{
		Status:               "UP",
		Service:              s.cfg.App.Name,
		Version:              s.version,
		AuthMode:             authMode,
		PasswordLoginEnabled: authMode == "password" && !strings.EqualFold(s.cfg.App.Environment, "production"),
		CasdoorAccountUrl:    s.cfg.Security.CasdoorAccountURL,
	}, nil
}

func (s *SystemService) Readiness(ctx context.Context, _ *forgev1.ReadinessRequest) (*forgev1.ReadinessResponse, error) {
	results := health.Run(ctx, s.checks)
	reply := &forgev1.ReadinessResponse{Status: "UP", Dependencies: make([]*forgev1.DependencyStatus, 0, len(results))}
	degraded := false
	for _, result := range results {
		dependency := &forgev1.DependencyStatus{Name: result.Name, Status: result.Status}
		if result.Status != "UP" {
			reply.Status = "DOWN"
			degraded = true
			dependency.Message = "dependency unavailable"
		}
		reply.Dependencies = append(reply.Dependencies, dependency)
	}
	if degraded {
		// Readiness is consumed by load balancers and orchestrators. Returning a
		// 503 keeps the transport status aligned with the dependency result;
		// callers must not mistake a JSON body with status=DOWN and HTTP 200 for
		// a ready instance.
		return reply, kratoserrors.ServiceUnavailable("DEPENDENCY_UNAVAILABLE", "one or more dependencies are unavailable")
	}
	return reply, nil
}

func (s *SystemService) GetSystemInfo(ctx context.Context, _ *forgev1.GetSystemInfoRequest) (*forgev1.GetSystemInfoResponse, error) {
	if _, ok := authn.Principal(ctx); !ok {
		return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	return &forgev1.GetSystemInfoResponse{
		Service: s.cfg.App.Name, Version: s.version, Environment: s.cfg.App.Environment,
		Region: s.cfg.App.Region, Zone: s.cfg.App.Zone, Providers: s.providers,
	}, nil
}
