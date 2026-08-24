package authz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"google.golang.org/grpc"
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

func TestPortalReadAllowsAuthenticatedUserWithoutRBACPermissions(t *testing.T) {
	handler := Server(PortalRules())(func(context.Context, any) (any, error) { return "ok", nil })
	operation := forgev1.OperationPortalServiceListPortalApplications

	if _, err := handler(authorizationContext(operation, domain.Principal{Type: "USER", UserID: "user-1"}), nil); err != nil {
		t.Fatalf("authenticated portal user without RBAC permissions was rejected: %v", err)
	}

	ctx := transport.NewServerContext(context.Background(), fakeTransport{operation: operation})
	if _, err := handler(ctx, nil); err == nil {
		t.Fatal("unauthenticated portal request was accepted")
	}
}

func TestPortalConsoleRequiresDedicatedPermission(t *testing.T) {
	handler := Server(PortalRules())(func(context.Context, any) (any, error) { return "ok", nil })
	operation := forgev1.OperationPortalServiceGetIdentityConsoleLink
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"iam.integration.read"}}), nil); err == nil {
		t.Fatal("identity reader was allowed to open the Casdoor console")
	}
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"iam.console.open"}}), nil); err != nil {
		t.Fatalf("identity console permission rejected: %v", err)
	}
}

func TestIdentityReaderCanListOnboardingApplicationsButCannotMutate(t *testing.T) {
	handler := Server(PortalRules())(func(context.Context, any) (any, error) { return "ok", nil })
	reader := domain.Principal{Permissions: []string{"iam.integration.read"}}
	if _, err := handler(authorizationContext(forgev1.OperationPortalServiceListAdminPortalApplications, reader), nil); err != nil {
		t.Fatalf("identity reader cannot list onboarding applications: %v", err)
	}
	if _, err := handler(authorizationContext(forgev1.OperationPortalServiceCreatePortalApplication, reader), nil); err == nil {
		t.Fatal("identity reader was allowed to create an application")
	}
}

func TestIdentityBindingMutationUsesIntegrationManageNotConsoleAccess(t *testing.T) {
	handler := Server(PortalRules())(func(context.Context, any) (any, error) { return "ok", nil })
	operation := forgev1.OperationPortalServiceUpsertApplicationIdentityBinding
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"iam.integration.manage"}}), nil); err != nil {
		t.Fatalf("identity integration manager was rejected without console access: %v", err)
	}
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"iam.console.open"}}), nil); err == nil {
		t.Fatal("console-only administrator was allowed to mutate identity bindings")
	}
}

func TestIntegrationTokenManagementRequiresDedicatedPermission(t *testing.T) {
	handler := Server(IdentityRules())(func(context.Context, any) (any, error) { return "ok", nil })
	operation := forgev1.OperationIdentityServiceListApiTokens
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"portal.application.manage"}}), nil); err == nil {
		t.Fatal("unrelated portal permission was allowed to list integration tokens")
	}
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"system.api_token.manage"}}), nil); err != nil {
		t.Fatalf("token permission rejected: %v", err)
	}
}

func TestAuditReadCompatibilityPermission(t *testing.T) {
	handler := Server(PlatformRules())(func(context.Context, any) (any, error) { return "ok", nil })
	operation := forgev1.OperationPlatformServiceListAuditLogs
	if _, err := handler(authorizationContext(operation, domain.Principal{Permissions: []string{"audit.read"}}), nil); err != nil {
		t.Fatalf("audit.read compatibility permission rejected: %v", err)
	}
}

func TestPlatformRulesRegisterUserEntitlementMutation(t *testing.T) {
	permissions, ok := PlatformRules()[forgev1.OperationPlatformServiceUpdateUserEntitlement]
	if !ok {
		t.Fatal("user entitlement mutation must have an authorization policy")
	}
	if len(permissions) != 1 || permissions[0] != "system.user.update" {
		t.Fatalf("unexpected permissions: %v", permissions)
	}
}

func TestPlatformRulesRegisterUserDetailRead(t *testing.T) {
	permissions, ok := PlatformRules()[forgev1.OperationPlatformServiceGetUser]
	if !ok {
		t.Fatal("user detail read must have an authorization policy")
	}
	if len(permissions) != 1 || permissions[0] != "system.user.read" {
		t.Fatalf("unexpected permissions: %v", permissions)
	}
}

func TestAuthorizationRulesCoverEveryGovernedRPC(t *testing.T) {
	rules := Rules()
	services := []struct {
		name    string
		methods []string
	}{
		{name: forgev1.PlatformService_ServiceDesc.ServiceName, methods: grpcMethodNames(forgev1.PlatformService_ServiceDesc.Methods)},
		{name: forgev1.ApprovalService_ServiceDesc.ServiceName, methods: grpcMethodNames(forgev1.ApprovalService_ServiceDesc.Methods)},
		{name: forgev1.PortalService_ServiceDesc.ServiceName, methods: grpcMethodNames(forgev1.PortalService_ServiceDesc.Methods)},
	}
	for _, service := range services {
		for _, method := range service.methods {
			operation := "/" + service.name + "/" + method
			if operation == forgev1.OperationPortalServiceConsumeApplicationEnrollment {
				continue
			}
			if _, ok := rules[operation]; !ok {
				t.Errorf("missing authorization policy for %s", operation)
			}
		}
	}
}

func grpcMethodNames(methods []grpc.MethodDesc) []string {
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		out = append(out, method.MethodName)
	}
	return out
}
