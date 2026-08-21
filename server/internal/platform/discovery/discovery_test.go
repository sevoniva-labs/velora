package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

type fakeNamingClient struct {
	services     map[string]model.Service
	registered   []vo.RegisterInstanceParam
	deregistered []vo.DeregisterInstanceParam
	onRegister   func()
}

func (f *fakeNamingClient) RegisterInstance(param vo.RegisterInstanceParam) (bool, error) {
	f.registered = append(f.registered, param)
	if f.onRegister != nil {
		f.onRegister()
	}
	return true, nil
}
func (f *fakeNamingClient) DeregisterInstance(param vo.DeregisterInstanceParam) (bool, error) {
	f.deregistered = append(f.deregistered, param)
	return true, nil
}
func (f *fakeNamingClient) GetService(param vo.GetServiceParam) (model.Service, error) {
	return f.services[param.ServiceName], nil
}

func TestBuildEndpointsSeparatesProtocols(t *testing.T) {
	endpoints := buildEndpoints(config.Discovery{
		ServiceName: "account-http", GRPCServiceName: "account-grpc",
		AdvertisePort: 8080, AdvertiseGRPCPort: 9090,
	}, "account", map[string]string{"version": "v1"})
	if len(endpoints) != 2 {
		t.Fatalf("endpoint count = %d, want 2", len(endpoints))
	}
	if endpoints[0].service != "account-http" || endpoints[0].metadata["protocol"] != "http" || endpoints[0].metadata["port"] != "8080" {
		t.Fatalf("unexpected HTTP endpoint: %#v", endpoints[0])
	}
	if endpoints[1].service != "account-grpc" || endpoints[1].metadata["protocol"] != "grpc" || endpoints[1].metadata["port"] != "9090" {
		t.Fatalf("unexpected gRPC endpoint: %#v", endpoints[1])
	}
	endpoints[0].metadata["protocol"] = "changed"
	if endpoints[1].metadata["protocol"] != "grpc" {
		t.Fatal("endpoint metadata maps must not alias")
	}
}

func TestBuildEndpointsUsesProtocolSpecificDefaults(t *testing.T) {
	endpoints := buildEndpoints(config.Discovery{AdvertisePort: 8080, AdvertiseGRPCPort: 9090}, "forge", nil)
	if endpoints[0].service != "forge-http" || endpoints[1].service != "forge-grpc" {
		t.Fatalf("unexpected default services: %#v", endpoints)
	}
}

func TestNacosPingRequiresOwnHealthyRegisteredEndpoints(t *testing.T) {
	cfg := config.Discovery{AdvertiseIP: "10.20.30.40", Cluster: "HZ-A", Group: "FORGE"}
	endpoints := buildEndpoints(config.Discovery{
		ServiceName: "account-http", GRPCServiceName: "account-grpc",
		AdvertisePort: 8080, AdvertiseGRPCPort: 9090,
	}, "account", map[string]string{"application": "account", "version": "v1"})
	services := make(map[string]model.Service, len(endpoints))
	for _, item := range endpoints {
		services[item.service] = model.Service{Hosts: []model.Instance{{
			Ip: cfg.AdvertiseIP, Port: item.port, Enable: true, Healthy: true, Ephemeral: true,
			Metadata: item.metadata,
		}}}
	}
	registry := &nacosRegistry{client: &fakeNamingClient{services: services}, cfg: cfg, endpoints: endpoints}
	registry.registered.Store(true)
	if err := registry.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() healthy endpoint error = %v", err)
	}

	unhealthy := services["account-grpc"]
	unhealthy.Hosts[0].Healthy = false
	services["account-grpc"] = unhealthy
	if err := registry.Ping(context.Background()); err == nil || !strings.Contains(err.Error(), "does not contain the healthy registered endpoint") {
		t.Fatalf("Ping() unhealthy endpoint error = %v", err)
	}

	services["account-grpc"] = model.Service{}
	if err := registry.Ping(context.Background()); err == nil {
		t.Fatal("Ping() accepted a service after its own instance was removed")
	}
}

func TestNacosRegisterRollsBackPartialRegistrationOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &fakeNamingClient{}
	client.onRegister = func() {
		if len(client.registered) == 1 {
			cancel()
		}
	}
	registry := &nacosRegistry{
		client: client,
		cfg: config.Discovery{
			AdvertiseIP: "10.20.30.40", Cluster: "HZ-A", Group: "FORGE", Weight: 1,
		},
		endpoints: []endpoint{
			{service: "account-http", port: 8080},
			{service: "account-grpc", port: 9090},
		},
	}

	err := registry.Register(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Register() error = %v, want context cancellation", err)
	}
	if len(client.registered) != 1 || len(client.deregistered) != 1 {
		t.Fatalf("registered=%d deregistered=%d, want a rolled back partial registration", len(client.registered), len(client.deregistered))
	}
	if client.deregistered[0].ServiceName != "account-http" || registry.registered.Load() {
		t.Fatalf("rollback = %#v, registered state = %t", client.deregistered, registry.registered.Load())
	}
}
