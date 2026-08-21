package kratosapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

func newTestBridge(t *testing.T) (*SessionBridge, cache.Cache) {
	t.Helper()
	c, err := cache.New(config.Cache{Provider: "memory", Prefix: "test:"})
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := NewSessionBridge(c, "https://auth.example.test/account", true, http.SameSiteLaxMode)
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
	if res.Code != http.StatusSeeOther || res.Header().Get("Location") != "/home?from=login" {
		t.Fatalf("bridge response status=%d location=%q", res.Code, res.Header().Get("Location"))
	}
	cookie := res.Result().Cookies()
	if len(cookie) != 1 || cookie[0].Name != casdoorSessionCookie || cookie[0].Value != "casdoor-cookie" || cookie[0].Domain != "" || !cookie[0].HttpOnly || !cookie[0].Secure {
		t.Fatalf("unexpected bridge cookie: %#v", cookie)
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
