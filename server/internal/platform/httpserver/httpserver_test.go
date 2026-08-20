package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestSPARejectsAPIAndTraversalFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("index"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := SPA(SPAOptions{Root: root})
	for _, path := range []string{"/api/v1/missing", "/../../etc/passwd"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "index") {
			t.Fatalf("path %s returned status=%d body=%q", path, response.Code, response.Body.String())
		}
	}
}

func TestSPAServesScriptStrictCSPAndGovernedCacheHeaders(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"index.html":               `<meta name="forge-csp-nonce" content="` + cspNonceMarker + `"><div>index</div>`,
		"runtime-config.js":        "window.__VELORA_CONFIG__ = {}",
		"assets/chunk-AbCd1234.js": "console.log('hashed')",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := SPA(SPAOptions{Root: root, FrameSources: []string{"https://remote.example.cn"}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/platform/users", nil))
	csp := response.Header().Get("Content-Security-Policy")
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), cspNonceMarker) {
		t.Fatalf("index response status=%d body=%q", response.Code, response.Body.String())
	}
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("strict CSP script policy missing: %q", csp)
	}
	if !strings.Contains(csp, "style-src-elem 'self' 'unsafe-inline'") || !strings.Contains(csp, "frame-src https://remote.example.cn") {
		t.Fatalf("governed style exception or frame source missing: %q", csp)
	}
	if !strings.Contains(response.Body.String(), "content=\"") || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("index security marker/cache policy missing")
	}

	for path, want := range map[string]string{
		"/runtime-config.js":        "no-store",
		"/assets/chunk-AbCd1234.js": "public, max-age=31536000, immutable",
	} {
		assetResponse := httptest.NewRecorder()
		handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, path, nil))
		if got := assetResponse.Header().Get("Cache-Control"); got != want {
			t.Errorf("path %s Cache-Control=%q want=%q", path, got, want)
		}
	}
}

func TestSPAWujieCSPRequiresExplicitUnsafeInlineMode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte(cspNonceMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := SPA(SPAOptions{Root: root, WujieCSPEnabled: true})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := response.Header().Get("Content-Security-Policy")
	for _, required := range []string{
		"frame-src 'self'",
		"script-src 'self' 'unsafe-inline'",
		"style-src 'self' 'unsafe-inline'",
	} {
		if !strings.Contains(csp, required) {
			t.Fatalf("Wujie CSP missing %q: %q", required, csp)
		}
	}
	if strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("Wujie CSP must not enable unsafe-eval: %q", csp)
	}
}

func TestCORSDeniesUntrustedOriginAndAllowsSameOrigin(t *testing.T) {
	handler := cors(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	tests := []struct {
		origin string
		want   int
	}{{"https://evil.example", http.StatusForbidden}, {"https://velora.example", http.StatusNoContent}}
	for _, item := range tests {
		request := httptest.NewRequest(http.MethodPost, "https://velora.example/api/v1/auth/login", nil)
		request.Header.Set("Origin", item.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != item.want {
			t.Errorf("origin %s status=%d want=%d", item.origin, response.Code, item.want)
		}
	}
}

func TestCORSDoesNotTrustForwardedProtoFromClient(t *testing.T) {
	handler := cors(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	request := httptest.NewRequest(http.MethodGet, "http://velora.example/api/v1/system/health", nil)
	request.Header.Set("Origin", "https://velora.example")
	request.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("forged X-Forwarded-Proto bypassed CORS: status=%d", response.Code)
	}
}

func TestCORSAllowsComposeNginxLoopbackOrigins(t *testing.T) {
	handler := cors([]string{"http://localhost:8443", "http://127.0.0.1:8443"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, origin := range []string{"http://localhost:8443", "http://127.0.0.1:8443"} {
		request := httptest.NewRequest(http.MethodGet, "http://server:8080/api/v1/system/health", nil)
		request.Header.Set("Origin", origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Fatalf("origin %s was not allowed: status=%d allow=%q", origin, response.Code, response.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}

func TestClientIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	var got string
	handler := clientIP([]string{"10.0.0.0/8"})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIP(r.Context()) }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.5")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if got != "192.0.2.10" {
		t.Fatalf("untrusted peer forwarded client IP = %q", got)
	}
}

func TestClientIPWalksTrustedForwardedChainFromRight(t *testing.T) {
	var got string
	var trusted bool
	handler := clientIP([]string{"10.0.0.0/8"})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIP(r.Context())
		trusted = IsTrustedProxy(r.Context())
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.8:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.5, 10.0.0.9")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if got != "198.51.100.5" || !trusted {
		t.Fatalf("trusted forwarded chain client IP = %q", got)
	}
}

func TestRequestIDReplacesUnsafeInput(t *testing.T) {
	handler := requestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "bad\r\nvalue" {
			t.Fatal("unsafe request ID retained")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(requestIDHeader, "bad\r\nvalue")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !validRequestID(response.Header().Get(requestIDHeader)) {
		t.Fatalf("generated request ID is invalid: %q", response.Header().Get(requestIDHeader))
	}
}

func TestMetricRouteBoundsCardinality(t *testing.T) {
	if got := metricRoute("/api/v1/admin/users/user-controlled-id"); got != "/api/v1/admin/users/:id" {
		t.Fatalf("metricRoute() = %q", got)
	}
	if got := metricRoute("/attacker/unique/path"); got != "unmatched" {
		t.Fatalf("metricRoute() = %q", got)
	}
	if got := metricRoute("/api/v1/auth/federated/oidc/casdoor/callback"); got != "/api/v1/auth/federated/oidc/:provider" {
		t.Fatalf("OIDC metricRoute() = %q", got)
	}
	if got := metricRoute("/api/v1/auth/forward/app-123"); got != "/api/v1/auth/forward/:application_id" {
		t.Fatalf("ForwardAuth metricRoute() = %q", got)
	}
}

func TestTracingCreatesServerSpan(t *testing.T) {
	previous := otel.GetTracerProvider()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})
	var spanContext trace.SpanContext
	handler := tracing("forge")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spanContext = trace.SpanFromContext(r.Context()).SpanContext()
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if !spanContext.IsValid() || !spanContext.IsSampled() {
		t.Fatalf("invalid server span context: %v", spanContext)
	}
}
