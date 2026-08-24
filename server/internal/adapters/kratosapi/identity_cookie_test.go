package kratosapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/ratelimit"
)

type acceptingTurnstile struct{}

func (acceptingTurnstile) Verify(context.Context, string, string) error { return nil }

type cookieHeader http.Header

func (h cookieHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h cookieHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h cookieHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h cookieHeader) Keys() []string             { return []string{} }
func (h cookieHeader) Values(key string) []string { return http.Header(h).Values(key) }

type cookieTransport struct {
	request cookieHeader
	reply   cookieHeader
}

func (c cookieTransport) Kind() transport.Kind            { return transport.KindHTTP }
func (c cookieTransport) Endpoint() string                { return "http://localhost" }
func (c cookieTransport) Operation() string               { return "" }
func (c cookieTransport) RequestHeader() transport.Header { return c.request }
func (c cookieTransport) ReplyHeader() transport.Header   { return c.reply }

func TestIdentityLoginCookiesAreHardened(t *testing.T) {
	reply := cookieHeader{}
	ctx := transport.NewServerContext(context.Background(), cookieTransport{request: cookieHeader{}, reply: reply})
	service := &IdentityService{secure: true, sameSite: http.SameSiteStrictMode}
	service.setLoginCookies(ctx, "session-secret", "csrf-secret", time.Now().Add(time.Hour))
	cookies := reply.Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("Set-Cookie count = %d, want 2", len(cookies))
	}
	if !strings.Contains(cookies[0], "HttpOnly") || !strings.Contains(cookies[0], "Secure") || !strings.Contains(cookies[0], "SameSite=Strict") {
		t.Fatalf("session cookie is not hardened: %s", cookies[0])
	}
	if strings.Contains(cookies[1], "HttpOnly") || !strings.Contains(cookies[1], "Secure") {
		t.Fatalf("CSRF cookie attributes are invalid: %s", cookies[1])
	}
}

func TestRequestCookieParsesSession(t *testing.T) {
	ctx := transport.NewServerContext(context.Background(), cookieTransport{
		request: cookieHeader{"Cookie": {"other=value; velora_session=session-secret"}}, reply: cookieHeader{},
	})
	if got := requestCookie(ctx, sessionCookieName); got != "session-secret" {
		t.Fatalf("requestCookie() = %q", got)
	}
}

func TestIdentitySecurityRateLimitRejectsSixthAttempt(t *testing.T) {
	service := &IdentityService{limiter: ratelimit.New(nil)}
	for attempt := 1; attempt <= 6; attempt++ {
		err := service.allow(context.Background(), "password-change:user:u1", 5, 15*time.Minute, "900")
		if attempt <= 5 && err != nil {
			t.Fatalf("attempt %d unexpectedly rejected: %v", attempt, err)
		}
		if attempt == 6 && (err == nil || kratoserrors.Reason(err) != "RATE_LIMITED") {
			t.Fatalf("attempt 6 error = %v, want RATE_LIMITED", err)
		}
	}
}

func TestLoginRateLimitKeysSeparateIPAndNormalizedAccount(t *testing.T) {
	keys := loginRateLimitKeys("192.0.2.10", " Alice ")
	if len(keys) != 2 || keys[0] != "192.0.2.10|login-ip" {
		t.Fatalf("unexpected login keys: %#v", keys)
	}
	if keys[1] == "" || strings.Contains(keys[1], "Alice") {
		t.Fatalf("account key leaks login identifier: %q", keys[1])
	}
	if same := loginRateLimitKeys("198.51.100.2", "alice"); same[1] != keys[1] {
		t.Fatalf("case/whitespace normalization mismatch: %q vs %q", keys[1], same[1])
	}
	if other := loginRateLimitKeys("198.51.100.2", "bob"); other[1] == keys[1] {
		t.Fatal("different accounts share a rate-limit key")
	}
}

func TestLoginChallengeIsRiskTriggeredAndClearedAfterSuccess(t *testing.T) {
	challengeCache, err := cache.New(config.Cache{Provider: "memory", Prefix: "identity-test:"})
	if err != nil {
		t.Fatal(err)
	}
	service := &IdentityService{turnstile: acceptingTurnstile{}, loginChallengeCache: challengeCache}
	ctx := context.Background()
	if required, err := service.loginChallengeRequired(ctx, "192.0.2.10", "alice"); err != nil || required {
		t.Fatalf("initial challenge = %v, err = %v; want false", required, err)
	}
	if err := service.markLoginChallenge(ctx, "192.0.2.10", "alice"); err != nil {
		t.Fatal(err)
	}
	if required, err := service.loginChallengeRequired(ctx, "192.0.2.10", "alice"); err != nil || !required {
		t.Fatalf("marked challenge = %v, err = %v; want true", required, err)
	}
	// IP and normalized account are independent risk signals.
	if required, err := service.loginChallengeRequired(ctx, "198.51.100.20", " ALICE "); err != nil || !required {
		t.Fatalf("account challenge = %v, err = %v; want true", required, err)
	}
	service.clearLoginChallenge(ctx, "192.0.2.10", "alice")
	if required, err := service.loginChallengeRequired(ctx, "192.0.2.10", "alice"); err != nil || required {
		t.Fatalf("cleared challenge = %v, err = %v; want false", required, err)
	}
}

func TestLoginChallengeFailsClosedWithoutSharedState(t *testing.T) {
	service := &IdentityService{turnstile: acceptingTurnstile{}}
	if _, err := service.loginChallengeRequired(context.Background(), "192.0.2.10", "alice"); err == nil {
		t.Fatal("loginChallengeRequired() error = nil, want unavailable cache error")
	}
}
