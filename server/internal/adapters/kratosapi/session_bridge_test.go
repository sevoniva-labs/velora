package kratosapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

func newTestBridge(t *testing.T) (*SessionBridge, cache.Cache) {
	t.Helper()
	c, err := cache.New(config.Cache{Provider: "memory", Prefix: "test:"})
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := NewSessionBridge(c, &database.DB{}, "https://auth.example.test/account", "https://home.example.test", true, http.SameSiteLaxMode)
	if err != nil {
		t.Fatal(err)
	}
	if bridge.ActionURL() != "https://auth.example.test/_velora/session/bridge" {
		t.Fatalf("bridge action URL=%q", bridge.ActionURL())
	}
	return bridge, c
}

func TestSessionBridgeConsumesTicketAndSetsHostOnlyCookie(t *testing.T) {
	bridge, c := newTestBridge(t)
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/home?from=login")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"ticket": {ticket}}
	req := httptest.NewRequest(http.MethodPost, "https://auth.example.test/_velora/session/bridge", strings.NewReader(form.Encode()))
	req.Host = "auth.example.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != "https://home.example.test/home?from=login" {
		t.Fatalf("bridge response status=%d location=%q", res.Code, res.Header().Get("Location"))
	}
	cookies := res.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != casdoorSessionCookie || cookies[0].Value != "casdoor-cookie" || cookies[0].Domain != "" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[1].Name != gatewaySessionCookie || cookies[1].Value == "" || !cookies[1].HttpOnly || !cookies[1].Secure {
		t.Fatalf("unexpected bridge cookies: %#v", cookies)
	}
	if got := res.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control=%q", got)
	}
	second := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(second, req)
	if second.Code != http.StatusGone {
		t.Fatalf("ticket replay status=%d", second.Code)
	}
	_ = c.Close()
}

func TestSessionBridgeRejectsInvalidPortalURL(t *testing.T) {
	c, err := cache.New(config.Cache{Provider: "memory", Prefix: "test:"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := NewSessionBridge(c, &database.DB{}, "https://auth.example.test/account", "/relative", true, http.SameSiteLaxMode); err == nil {
		t.Fatal("expected invalid portal URL to fail")
	}
}

func TestSessionBridgeReturnsAuthorizationContinuationToAuthHost(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer c.Close()
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/login/oauth/authorize?client_id=spectra&state=opaque")
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"ticket": {ticket}}
	req := httptest.NewRequest(http.MethodPost, "https://auth.example.test/_velora/session/bridge", strings.NewReader(form.Encode()))
	req.Host = "auth.example.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(res, req)
	if got := res.Header().Get("Location"); got != "https://auth.example.test/login/oauth/authorize?client_id=spectra&state=opaque" {
		t.Fatalf("authorization continuation=%q", got)
	}
}

func TestAuthorizationGatewayKeepsCasdoorBehindPortalLogin(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer c.Close()
	bridge.resolveApplication = func(context.Context, string) (authorizationApplication, error) {
		return authorizationApplication{Code: "spectra", Name: "Spectra", RedirectURIs: []string{"https://spectra.example.test/api/v1/auth/oidc/callback"}}, nil
	}
	rawQuery := "client_id=spectra-client&code_challenge=challenge&code_challenge_method=S256&redirect_uri=https%3A%2F%2Fspectra.example.test%2Fapi%2Fv1%2Fauth%2Foidc%2Fcallback&response_type=code&scope=openid&state=opaque"

	unauthenticated := httptest.NewRequest(http.MethodGet, "https://auth.example.test/_velora/authorize?"+rawQuery, nil)
	unauthenticated.Host = "auth.example.test"
	unauthenticatedResult := httptest.NewRecorder()
	bridge.AuthorizationHandler().ServeHTTP(unauthenticatedResult, unauthenticated)
	if unauthenticatedResult.Code != http.StatusFound {
		t.Fatalf("unauthenticated status=%d", unauthenticatedResult.Code)
	}
	location, err := url.Parse(unauthenticatedResult.Header().Get("Location"))
	if err != nil || location.Host != "home.example.test" || location.Path != "/login" || location.Query().Get("app") != "spectra" || location.Query().Get("redirect") != "/login/oauth/authorize?"+rawQuery {
		t.Fatalf("portal redirect=%q err=%v", location, err)
	}

	token, err := cache.RandomToken(32)
	if err != nil || c.Set(context.Background(), gatewaySessionKey(token), "active", time.Minute) != nil {
		t.Fatal("failed to seed gateway session")
	}
	authenticated := httptest.NewRequest(http.MethodGet, "https://auth.example.test/_velora/authorize?"+rawQuery, nil)
	authenticated.Host = "auth.example.test"
	authenticated.AddCookie(&http.Cookie{Name: gatewaySessionCookie, Value: token})
	authenticatedResult := httptest.NewRecorder()
	bridge.AuthorizationHandler().ServeHTTP(authenticatedResult, authenticated)
	if authenticatedResult.Code != http.StatusOK || authenticatedResult.Header().Get("X-Accel-Redirect") != casdoorAuthorizeInternalPath+"?"+rawQuery {
		t.Fatalf("authorized status=%d internal_redirect=%q", authenticatedResult.Code, authenticatedResult.Header().Get("X-Accel-Redirect"))
	}
}

func TestSessionBridgeRejectsQueryTicketsAndUntrustedHost(t *testing.T) {
	bridge, c := newTestBridge(t)
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{
		"https://auth.example.test/_velora/session/bridge?ticket=" + url.QueryEscape(ticket),
		"https://evil.example.test/_velora/session/bridge",
	} {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader("ticket="+url.QueryEscape(ticket)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res := httptest.NewRecorder()
		bridge.Handler().ServeHTTP(res, req)
		if res.Code != http.StatusNotFound {
			t.Fatalf("target %s status=%d", target, res.Code)
		}
	}
	_ = c.Close()
}
