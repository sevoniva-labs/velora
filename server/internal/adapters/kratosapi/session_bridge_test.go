package kratosapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

var testGatewayPrincipal = domain.Principal{Type: "USER", UserID: "user-1", OrganizationID: "org-1", SessionID: "session-1", Roles: []string{"user"}}

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
	bridge.ConfigureAccessControl(func(_ context.Context, sessionID string) (domain.Principal, error) {
		if sessionID != testGatewayPrincipal.SessionID {
			return domain.Principal{}, errors.New("session not found")
		}
		return testGatewayPrincipal, nil
	}, func(context.Context, domain.Principal, string) error { return nil })
	return bridge, c
}

func TestSessionBridgeConsumesTicketAndSetsHostOnlyCookie(t *testing.T) {
	bridge, c := newTestBridge(t)
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/home?from=login", testGatewayPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"ticket": {ticket}}
	req := httptest.NewRequest(http.MethodPost, "https://auth.example.test/_velora/session/bridge", strings.NewReader(form.Encode()))
	req.Host = "auth.example.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://home.example.test")
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

func TestSessionBridgeRejectsCrossSiteTicketConsumption(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer func() { _ = c.Close() }()
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/", testGatewayPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"ticket": {ticket}}
	for _, origin := range []string{"", "https://evil.example.test"} {
		req := httptest.NewRequest(http.MethodPost, "https://auth.example.test/_velora/session/bridge", strings.NewReader(form.Encode()))
		req.Host = "auth.example.test"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			req.Header.Set("Origin", origin)
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		res := httptest.NewRecorder()
		bridge.Handler().ServeHTTP(res, req)
		if res.Code != http.StatusForbidden {
			t.Fatalf("origin %q status=%d", origin, res.Code)
		}
	}
}

func TestSessionBridgeBindsAuthorizationTicketToInitiatingBrowser(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer func() { _ = c.Close() }()
	returnPath := "/login/oauth/authorize?client_id=spectra&state=opaque&" + bridgeNonceParam + "=browser-a"
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", returnPath, testGatewayPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"ticket": {ticket}}
	request := func(nonce string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "https://auth.example.test/_velora/session/bridge", strings.NewReader(form.Encode()))
		req.Host = "auth.example.test"
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://home.example.test")
		if nonce != "" {
			req.AddCookie(&http.Cookie{Name: bridgeNonceCookie, Value: nonce})
		}
		return req
	}

	wrongBrowser := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(wrongBrowser, request("browser-b"))
	if wrongBrowser.Code != http.StatusForbidden {
		t.Fatalf("wrong browser status=%d", wrongBrowser.Code)
	}

	initiatingBrowser := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(initiatingBrowser, request("browser-a"))
	if initiatingBrowser.Code != http.StatusSeeOther {
		t.Fatalf("initiating browser status=%d", initiatingBrowser.Code)
	}
	location, err := url.Parse(initiatingBrowser.Header().Get("Location"))
	if err != nil || location.Query().Has(bridgeNonceParam) {
		t.Fatalf("bridge nonce leaked to provider: location=%q err=%v", location, err)
	}
	if cookies := initiatingBrowser.Result().Cookies(); len(cookies) != 3 || cookies[2].Name != bridgeNonceCookie || cookies[2].MaxAge != -1 {
		t.Fatalf("browser nonce was not cleared: %#v", cookies)
	}
}

func TestSessionBridgeRejectsInvalidPortalURL(t *testing.T) {
	c, err := cache.New(config.Cache{Provider: "memory", Prefix: "test:"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := NewSessionBridge(c, &database.DB{}, "https://auth.example.test/account", "/relative", true, http.SameSiteLaxMode); err == nil {
		t.Fatal("expected invalid portal URL to fail")
	}
}

func TestSessionBridgeReturnsAuthorizationContinuationToAuthHost(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer func() { _ = c.Close() }()
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/login/oauth/authorize?client_id=spectra&state=opaque", testGatewayPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"ticket": {ticket}}
	req := httptest.NewRequest(http.MethodPost, "https://auth.example.test/_velora/session/bridge", strings.NewReader(form.Encode()))
	req.Host = "auth.example.test"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://home.example.test")
	res := httptest.NewRecorder()
	bridge.Handler().ServeHTTP(res, req)
	if got := res.Header().Get("Location"); got != "https://auth.example.test/login/oauth/authorize?client_id=spectra&state=opaque" {
		t.Fatalf("authorization continuation=%q", got)
	}
}

func TestAuthorizationGatewayKeepsCasdoorBehindPortalLogin(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer func() { _ = c.Close() }()
	bridge.resolveApplication = func(_ context.Context, _ string, organizationID string) (authorizationApplication, error) {
		if organizationID != "" && organizationID != testGatewayPrincipal.OrganizationID {
			return authorizationApplication{}, errors.New("organization mismatch")
		}
		return authorizationApplication{ID: "app-1", OrganizationID: "org-1", Code: "spectra", Name: "Spectra", RedirectURIs: []string{"https://spectra.example.test/api/v1/auth/oidc/callback"}}, nil
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
	continuation, continuationErr := url.Parse(location.Query().Get("redirect"))
	if err != nil || continuationErr != nil || location.Host != "home.example.test" || location.Path != "/login" || location.Query().Get("app") != "spectra" || continuation.Path != "/login/oauth/authorize" || continuation.Query().Get(bridgeNonceParam) == "" || continuation.Query().Get("state") != "opaque" {
		t.Fatalf("portal redirect=%q err=%v", location, err)
	}
	if cookies := unauthenticatedResult.Result().Cookies(); len(cookies) != 2 || cookies[1].Name != bridgeNonceCookie || cookies[1].Value == "" || !cookies[1].HttpOnly || !cookies[1].Secure {
		t.Fatalf("authorization browser nonce cookie=%#v", cookies)
	}

	token, err := cache.RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := json.Marshal(gatewaySession{UserID: testGatewayPrincipal.UserID, OrganizationID: testGatewayPrincipal.OrganizationID, SessionID: testGatewayPrincipal.SessionID})
	if err != nil || c.Set(context.Background(), gatewaySessionKey(token), string(linked), time.Minute) != nil {
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

func TestAuthorizationGatewayRevalidatesApplicationAccess(t *testing.T) {
	bridge, c := newTestBridge(t)
	defer func() { _ = c.Close() }()
	bridge.resolveApplication = func(context.Context, string, string) (authorizationApplication, error) {
		return authorizationApplication{ID: "app-1", OrganizationID: "org-1", Code: "spectra", Name: "Spectra", RedirectURIs: []string{"https://spectra.example.test/callback"}}, nil
	}
	bridge.authorizeApp = func(context.Context, domain.Principal, string) error { return errors.New("access denied") }
	token, err := cache.RandomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := json.Marshal(gatewaySession{UserID: testGatewayPrincipal.UserID, OrganizationID: testGatewayPrincipal.OrganizationID, SessionID: testGatewayPrincipal.SessionID})
	if err != nil || c.Set(context.Background(), gatewaySessionKey(token), string(linked), time.Minute) != nil {
		t.Fatal("failed to seed gateway session")
	}
	rawQuery := "client_id=spectra-client&code_challenge=challenge&code_challenge_method=S256&redirect_uri=https%3A%2F%2Fspectra.example.test%2Fcallback&response_type=code&scope=openid&state=opaque"
	req := httptest.NewRequest(http.MethodGet, "https://auth.example.test/_velora/authorize?"+rawQuery, nil)
	req.Host = "auth.example.test"
	req.AddCookie(&http.Cookie{Name: gatewaySessionCookie, Value: token})
	res := httptest.NewRecorder()
	bridge.AuthorizationHandler().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden || res.Header().Get("X-Accel-Redirect") != "" {
		t.Fatalf("status=%d internal_redirect=%q", res.Code, res.Header().Get("X-Accel-Redirect"))
	}
}

func TestSessionBridgeRejectsQueryTicketsAndUntrustedHost(t *testing.T) {
	bridge, c := newTestBridge(t)
	ticket, err := bridge.Create(context.Background(), "casdoor-cookie", "/", testGatewayPrincipal)
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
