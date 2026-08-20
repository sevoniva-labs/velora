package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-ID"

type FilterOptions struct {
	Log            *slog.Logger
	Metrics        *metrics.Metrics
	Secure         bool
	AllowedOrigins []string
	MaxBodyBytes   int64
	ServiceName    string
}

func Filters(options FilterOptions) []khttp.FilterFunc {
	return []khttp.FilterFunc{
		recoverer(options.Log),
		requestID,
		tracing(options.ServiceName),
		securityHeaders(options.Secure),
		cors(options.AllowedOrigins),
		bodyLimit(options.MaxBodyBytes),
		accessLog(options.Log, options.Metrics),
	}
}

func tracing(serviceName string) khttp.FilterFunc {
	operation := strings.TrimSpace(serviceName) + ".http"
	if operation == ".http" {
		operation = "forge.http"
	}
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, operation,
			otelhttp.WithFilter(func(request *http.Request) bool {
				path := request.URL.Path
				return path != "/metrics" && path != "/api/v1/system/health" && path != "/api/v1/system/ready" && !strings.HasPrefix(path, "/debug/pprof/")
			}),
			otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
				return request.Method + " " + metricRoute(request.URL.Path)
			}),
		)
	}
}

func recoverer(log *slog.Logger) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error("HTTP panic recovered", "panic", fmt.Sprint(recovered), "request_id", RequestID(r.Context()), "stack", string(debug.Stack()))
					writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type requestIDContextKey struct{}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if !validRequestID(id) {
			id = uuid.NewString()
		}
		r.Header.Set(requestIDHeader, id)
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id)))
	})
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("._:-", char) {
			continue
		}
		return false
	}
	return true
}

func securityHeaders(secure bool) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			header.Set("Referrer-Policy", "no-referrer")
			header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; object-src 'none'")
			if secure {
				header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cors(allowedOrigins []string) khttp.FilterFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
			if origin != "" {
				_, explicitlyAllowed := allowed[origin]
				if !explicitlyAllowed && !sameOrigin(origin, r) {
					writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
					return
				}
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sameOrigin(origin string, request *http.Request) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	scheme := "http"
	// Do not trust a client-supplied X-Forwarded-Proto header. Explicitly
	// configured AllowedOrigins cover TLS-terminated proxy deployments; using
	// the header here would let an untrusted caller forge same-origin status.
	if request.TLS != nil {
		scheme = "https"
	}
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(parsed.Host, request.Host)
}

func bodyLimit(maximum int64) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maximum > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maximum)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func accessLog(log *slog.Logger, met *metrics.Metrics) khttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			writer := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			var done func()
			if met != nil {
				done = met.Begin(r.Method)
				defer done()
			}
			next.ServeHTTP(writer, r)
			route := metricRoute(r.URL.Path)
			if met != nil {
				met.Observe(r.Method, route, writer.status, writer.bytes, start)
			}
			span := trace.SpanFromContext(r.Context()).SpanContext()
			log.Info("HTTP request", "method", r.Method, "route", route, "status", writer.status, "bytes", writer.bytes,
				"duration_ms", time.Since(start).Milliseconds(), "request_id", RequestID(r.Context()), "trace_id", span.TraceID().String())
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	written, err := w.ResponseWriter.Write(body)
	w.bytes += written
	return written, err
}

func (w *responseRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func metricRoute(path string) string {
	for _, prefix := range []string{
		"/api/v1/admin/users/", "/api/v1/admin/departments/", "/api/v1/admin/positions/", "/api/v1/admin/user-groups/", "/api/v1/admin/sessions/", "/api/v1/admin/roles/", "/api/v1/api-tokens/",
		"/api/v1/auth/federated/oidc/", "/api/v1/auth/forward/",
	} {
		if strings.HasPrefix(path, prefix) {
			if strings.HasPrefix(prefix, "/api/v1/auth/federated/oidc/") {
				return prefix + ":provider"
			}
			if strings.HasPrefix(prefix, "/api/v1/auth/forward/") {
				return prefix + ":application_id"
			}
			return prefix + ":id"
		}
	}
	for _, exact := range []string{
		"/api/v1/system/health", "/api/v1/system/ready", "/api/v1/system/info", "/api/v1/auth/login",
		"/api/v1/auth/logout", "/api/v1/auth/password", "/api/v1/me", "/api/v1/api-tokens",
		"/api/v1/admin/users", "/api/v1/admin/departments", "/api/v1/admin/positions", "/api/v1/admin/user-groups", "/api/v1/admin/organization", "/api/v1/admin/security-config",
		"/api/v1/admin/roles", "/api/v1/admin/permissions", "/api/v1/admin/sessions",
		"/api/v1/admin/audit-logs", "/api/v1/admin/audit-logs/export", "/metrics",
	} {
		if path == exact {
			return exact
		}
	}
	return "unmatched"
}

func writeError(w http.ResponseWriter, status int, reason, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": status, "reason": reason, "message": message})
}
