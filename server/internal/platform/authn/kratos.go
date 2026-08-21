package authn

import (
	"context"
	"errors"
	"net/http"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

type Authenticator interface {
	Authenticate(context.Context, string) (domain.Principal, error)
	AuthenticateAPIToken(context.Context, string) (domain.Principal, error)
}

type principalContextKey struct{}

func Server(authenticator Authenticator) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "transport context is required")
			}
			principal, err := authenticate(ctx, authenticator, tr.RequestHeader())
			if err != nil {
				return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication failed")
			}
			return next(WithPrincipal(ctx, principal), req)
		}
	}
}

func authenticate(ctx context.Context, authenticator Authenticator, header transport.Header) (domain.Principal, error) {
	authorization := strings.TrimSpace(header.Get("Authorization"))
	if authorization != "" {
		token := bearerToken(authorization)
		if token == "" {
			return domain.Principal{}, errors.New("unsupported authorization scheme")
		}
		return authenticator.AuthenticateAPIToken(ctx, token)
	}
	session := sessionCookie(header.Get("Cookie"))
	if session == "" {
		return domain.Principal{}, errors.New("credentials are required")
	}
	return authenticator.Authenticate(ctx, session)
}

func WithPrincipal(ctx context.Context, principal domain.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func Principal(ctx context.Context) (domain.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(domain.Principal)
	return principal, ok
}

func bearerToken(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func sessionCookie(value string) string {
	request := &http.Request{Header: http.Header{"Cookie": []string{value}}}
	cookie, err := request.Cookie("velora_session")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
