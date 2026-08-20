package csrf

import (
	"context"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
)

type testHeader map[string][]string

func (h testHeader) Get(key string) string {
	values := h.Values(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func (h testHeader) Set(key, value string) { h[key] = []string{value} }
func (h testHeader) Add(key, value string) { h[key] = append(h[key], value) }
func (h testHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
func (h testHeader) Values(key string) []string {
	for candidate, values := range h {
		if strings.EqualFold(candidate, key) {
			return values
		}
	}
	return nil
}

type testTransport struct {
	operation string
	header    testHeader
}

func (t testTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (t testTransport) Endpoint() string                { return "http://localhost" }
func (t testTransport) Operation() string               { return t.operation }
func (t testTransport) RequestHeader() transport.Header { return t.header }
func (t testTransport) ReplyHeader() transport.Header   { return testHeader{} }

func TestServerRequiresMatchingTokenForSessionWrites(t *testing.T) {
	tests := []struct {
		name      string
		principal domain.Principal
		header    testHeader
		wantError bool
	}{
		{name: "matching session token", principal: domain.Principal{Type: "SESSION"}, header: testHeader{"Cookie": {"velora_csrf=secret"}, "X-CSRF-Token": {"secret"}}},
		{name: "missing session token", principal: domain.Principal{Type: "SESSION"}, header: testHeader{"Cookie": {"velora_csrf=secret"}}, wantError: true},
		{name: "API token exempt", principal: domain.Principal{Type: "TOKEN"}, header: testHeader{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := transport.NewServerContext(context.Background(), testTransport{operation: forgev1.OperationIdentityServiceChangePassword, header: tt.header})
			ctx = authn.WithPrincipal(ctx, tt.principal)
			called := false
			handler := Server()(middleware.Handler(func(context.Context, any) (any, error) { called = true; return nil, nil }))
			_, err := handler(ctx, nil)
			if tt.wantError {
				if err == nil || kratoserrors.Reason(err) != "CSRF_MISMATCH" {
					t.Fatalf("error = %v, want CSRF_MISMATCH", err)
				}
				return
			}
			if err != nil || !called {
				t.Fatalf("handler called = %v, error = %v", called, err)
			}
		})
	}
}
