package authz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
)

type fakeHeader struct{}

func (fakeHeader) Get(string) string      { return "" }
func (fakeHeader) Set(string, string)     {}
func (fakeHeader) Add(string, string)     {}
func (fakeHeader) Keys() []string         { return nil }
func (fakeHeader) Values(string) []string { return nil }

type fakeTransport struct{ operation string }

func (f fakeTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (f fakeTransport) Endpoint() string                { return "grpc://127.0.0.1:9090" }
func (f fakeTransport) Operation() string               { return f.operation }
func (f fakeTransport) RequestHeader() transport.Header { return fakeHeader{} }
func (f fakeTransport) ReplyHeader() transport.Header   { return fakeHeader{} }

func authorizationContext(operation string, principal domain.Principal) context.Context {
	ctx := transport.NewServerContext(context.Background(), fakeTransport{operation: operation})
	return authn.WithPrincipal(ctx, principal)
}

func TestPlatformAuthorizationRequiresPermissionAndTokenScope(t *testing.T) {
	handler := Server(PlatformRules())(func(context.Context, any) (any, error) { return "ok", nil })
	operation := forgev1.OperationPlatformServiceListUsers
	allowed := domain.Principal{Type: "TOKEN", Permissions: []string{"system.user.read"}, Scopes: []string{"system.user.read"}}
	if _, err := handler(authorizationContext(operation, allowed), nil); err != nil {
		t.Fatalf("authorized principal rejected: %v", err)
	}
	for name, principal := range map[string]domain.Principal{
		"missing permission": {Type: "TOKEN", Scopes: []string{"system.user.read"}},
		"missing scope":      {Type: "TOKEN", Permissions: []string{"system.user.read"}, Scopes: []string{"system.audit.read"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := handler(authorizationContext(operation, principal), nil); err == nil {
				t.Fatal("unauthorized principal accepted")
			}
		})
	}
}

func TestPlatformAuthorizationDeniesUnregisteredOperation(t *testing.T) {
	handler := Server(PlatformRules())(func(context.Context, any) (any, error) { return "ok", nil })
	principal := domain.Principal{Roles: []string{"system_admin"}}
	ctx := authorizationContext("/forge.v1.PlatformService/FutureOperation", principal)
	if _, err := handler(ctx, nil); err == nil {
		t.Fatal("unregistered platform operation was accepted")
	}
}
