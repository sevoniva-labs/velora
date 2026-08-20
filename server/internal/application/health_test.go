package application

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockedHealthTarget(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"http://169.254.169.254/latest/meta-data", true}, // 云元数据
		{"http://169.254.169.254/computeMetadata/v1/", true},
		{"http://169.254.10.10/x", true},                // link-local
		{"http://localhost:8080/healthz", true},         // localhost
		{"http://127.0.0.1:9090/metrics", true},         // 回环
		{"http://[::1]/x", true},                        // IPv6 回环
		{"http://10.0.0.5:8080/healthz", false},         // 企业内网（允许）
		{"http://192.168.1.10/health", false},           // 私网（允许）
		{"https://app.example.com/healthz", false},      // 公网域名（允许）
		{"https://intranet.corp.example/health", false}, // 内网域名（允许）
		{"file:///etc/passwd", true},                    // 非法 scheme
		{"", true},                                      // 空
		{"not-a-url", true},                             // 非法
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, blockedHealthTarget(tc.url), "url=%q", tc.url)
	}
}

func TestHealthCheckerProbeBlocked(t *testing.T) {
	// 即使本机有可达的探测目标，link-local/localhost 一律 DOWN。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewHealthChecker(0)
	app := &Application{ID: 1, HealthCheckEnabled: true, HealthCheckURL: srv.URL}
	// httptest 返回 127.0.0.1:port → 应被拦截为 DOWN
	assert.Equal(t, HealthDown, h.Check(t.Context(), app), "回环地址探测应被拒绝")
}
