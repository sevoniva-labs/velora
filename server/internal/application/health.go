package application

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HealthChecker 提供基础 HTTP 健康检查（带 30s 内存缓存，避免放大请求）。
type HealthChecker struct {
	timeout time.Duration

	mu    sync.Mutex
	cache map[uint64]healthEntry
}

type healthEntry struct {
	status    string
	checkedAt time.Time
}

const healthCacheTTL = 30 * time.Second

// NewHealthChecker 创建健康检查器。
func NewHealthChecker(timeout time.Duration) *HealthChecker {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HealthChecker{
		timeout: timeout,
		cache:   map[uint64]healthEntry{},
	}
}

// Check 返回应用健康状态：
//   - 未启用健康检查 → UNKNOWN
//   - 启用但未配置 URL → DOWN
//   - HTTP 2xx → UP；其余 → DOWN；异常 → DOWN
func (h *HealthChecker) Check(ctx context.Context, app *Application) string {
	if !app.HealthCheckEnabled {
		return HealthUnknown
	}
	if app.HealthCheckURL == "" {
		return HealthDown
	}

	h.mu.Lock()
	if e, ok := h.cache[app.ID]; ok && time.Since(e.checkedAt) < healthCacheTTL {
		h.mu.Unlock()
		return e.status
	}
	h.mu.Unlock()

	// 探测使用独立 context：客户端断开不取消探测，也不受请求 deadline 影响。
	probeCtx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	status := h.probe(probeCtx, app.HealthCheckURL)

	h.mu.Lock()
	h.cache[app.ID] = healthEntry{status: status, checkedAt: time.Now()}
	h.mu.Unlock()
	return status
}

func (h *HealthChecker) probe(ctx context.Context, rawURL string) string {
	// 防 SSRF 放大器：普通用户刷新应用列表会触发服务端探测（缓存 30s）。
	// 拒绝 link-local / metadata（169.254.0.0/16，含云元数据 169.254.169.254）、
	// localhost / 回环地址；企业内网（10/8、172.16/12、192.168/16）保留（内网应用常见）。
	if blockedHealthTarget(rawURL) {
		return HealthDown
	}
	client := &http.Client{Timeout: h.timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return HealthDown
	}
	resp, err := client.Do(req)
	if err != nil {
		return HealthDown
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return HealthUp
	}
	return HealthDown
}

// blockedHealthTarget 判定健康检查目标是否属于禁止探测的地址段。
func blockedHealthTarget(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return true
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// 域名目标：不做 DNS 解析（避免探测放大），仅拦截字面回环；内网域名允许。
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// 显式拦截云元数据段（部分环境不按 link-local 判定）。
	return ip.Equal(net.ParseIP("169.254.169.254"))
}
