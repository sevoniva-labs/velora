package authn

import (
	"context"
	"errors"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

type fakeAuthenticator struct{ err error }

func (f fakeAuthenticator) Authenticate(context.Context, string) (domain.Principal, error) {
	if f.err != nil {
		return domain.Principal{}, f.err
	}
	return domain.Principal{Type: "SESSION", UserID: "user-1", OrganizationID: "org-1", SessionID: "session-1"}, nil
}

func (f fakeAuthenticator) AuthenticateAPIToken(context.Context, string) (domain.Principal, error) {
	if f.err != nil {
		return domain.Principal{}, f.err
	}
	return domain.Principal{Type: "TOKEN", UserID: "user-1", OrganizationID: "org-1"}, nil
}

type fakeHeader map[string]string

func (h fakeHeader) Get(key string) string      { return h[key] }
func (h fakeHeader) Set(key, value string)      { h[key] = value }
func (h fakeHeader) Add(key, value string)      { h[key] = value }
func (h fakeHeader) Keys() []string             { return nil }
func (h fakeHeader) Values(key string) []string { return []string{h[key]} }

type fakeTransport struct{ header fakeHeader }

func (f fakeTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (f fakeTransport) Endpoint() string                { return "grpc://127.0.0.1:9090" }
func (f fakeTransport) Operation() string               { return "/forge.v1.SystemService/GetSystemInfo" }
func (f fakeTransport) RequestHeader() transport.Header { return f.header }
func (f fakeTransport) ReplyHeader() transport.Header   { return fakeHeader{} }

func TestServerAuthenticatesBearerAndInjectsPrincipal(t *testing.T) {
	ctx := transport.NewServerContext(context.Background(), fakeTransport{header: fakeHeader{"Authorization": "Bearer machine-token"}})
	called := false
	handler := Server(fakeAuthenticator{})(func(ctx context.Context, _ any) (any, error) {
		principal, ok := Principal(ctx)
		called = ok && principal.UserID == "user-1"
		return "ok", nil
	})
	if _, err := handler(ctx, nil); err != nil || !called {
		t.Fatalf("authenticated handler failed: called=%v err=%v", called, err)
	}
}

func TestServerRejectsMissingOrInvalidBearer(t *testing.T) {
	for name, header := range map[string]string{"missing": "", "basic": "Basic abc"} {
		t.Run(name, func(t *testing.T) {
			ctx := transport.NewServerContext(context.Background(), fakeTransport{header: fakeHeader{"Authorization": header}})
			handler := Server(fakeAuthenticator{})(func(context.Context, any) (any, error) { return nil, nil })
			if _, err := handler(ctx, nil); err == nil {
				t.Fatal("unauthenticated request was accepted")
			}
		})
	}
	ctx := transport.NewServerContext(context.Background(), fakeTransport{header: fakeHeader{"Authorization": "Bearer invalid"}})
	handler := Server(fakeAuthenticator{err: errors.New("invalid")})(func(context.Context, any) (any, error) { return nil, nil })
	if _, err := handler(ctx, nil); err == nil {
		t.Fatal("invalid token was accepted")
	}
}

func TestServerAuthenticatesSessionCookie(t *testing.T) {
	ctx := transport.NewServerContext(context.Background(), fakeTransport{header: fakeHeader{"Cookie": "theme=light; velora_session=session-token"}})
	handler := Server(fakeAuthenticator{})(func(ctx context.Context, _ any) (any, error) {
		principal, ok := Principal(ctx)
		if !ok || principal.Type != "SESSION" || principal.SessionID != "session-1" {
			t.Fatalf("unexpected session principal: %+v", principal)
		}
		return nil, nil
	})
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("session cookie rejected: %v", err)
	}
}

func TestInvalidBearerDoesNotFallBackToCookie(t *testing.T) {
	ctx := transport.NewServerContext(context.Background(), fakeTransport{header: fakeHeader{
		"Authorization": "Basic invalid", "Cookie": "velora_session=session-token",
	}})
	handler := Server(fakeAuthenticator{})(func(context.Context, any) (any, error) {
		t.Fatal("credential confusion reached handler")
		return nil, nil
	})
	if _, err := handler(ctx, nil); err == nil {
		t.Fatal("invalid Authorization header fell back to session cookie")
	}
}
