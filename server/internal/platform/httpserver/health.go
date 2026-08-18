package httpserver

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

func newRequestID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// RegisterHealth 注册 /healthz /readyz /metrics。
func RegisterHealth(r *gin.Engine, db *gorm.DB) {
	r.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		if db == nil {
			response.ErrorWith(c, 503, "A05001", "数据库未初始化")
			return
		}
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			response.ErrorWith(c, 503, "A05001", "依赖不可用: postgres")
			return
		}
		response.OK(c, gin.H{"status": "ready", "dependencies": gin.H{"postgres": "up"}})
	})
	r.GET("/metrics", metricsHandler())
}

// metrics 输出基础 Prometheus 文本格式指标（不引入完整 Observability 栈）。
type counters struct {
	mu       sync.Mutex
	requests map[string]int64 // method:path → count
	started  time.Time
}

var metricsCounters = &counters{requests: map[string]int64{}, started: time.Now()}

func observeRequest(method, path string) {
	metricsCounters.mu.Lock()
	defer metricsCounters.mu.Unlock()
	metricsCounters.requests[method+":"+path]++
}

func metricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		metricsCounters.mu.Lock()
		defer metricsCounters.mu.Unlock()
		var sb []byte
		sb = append(sb, "# HELP velora_up Whether Velora is up.\n# TYPE velora_up gauge\nvelora_up 1\n"...)
		sb = append(sb, "# HELP velora_http_requests_total Total HTTP requests by method and path.\n# TYPE velora_http_requests_total counter\n"...)
		for k, v := range metricsCounters.requests {
			// k 为 "METHOD:path"，拆分后输出 method/path 两个 label（path 为路由模板，基数可控）。
			parts := strings.SplitN(k, ":", 2)
			method, path := parts[0], ""
			if len(parts) == 2 {
				path = parts[1]
			}
			sb = append(sb, "velora_http_requests_total{method=\""...)
			sb = append(sb, method...)
			sb = append(sb, "\",path=\""...)
			sb = append(sb, path...)
			sb = append(sb, "\"} "...)
			sb = appendInt(sb, v)
			sb = append(sb, '\n')
		}
		sb = append(sb, "# HELP velora_uptime_seconds Process uptime in seconds.\n# TYPE velora_uptime_seconds gauge\nvelora_uptime_seconds "...)
		sb = appendInt(sb, int64(time.Since(metricsCounters.started).Seconds()))
		sb = append(sb, '\n')
		c.Data(200, "text/plain; version=0.0.4; charset=utf-8", sb)
	}
}

func appendInt(b []byte, v int64) []byte {
	if v == 0 {
		return append(b, '0')
	}
	var digits [20]byte
	i := len(digits)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return append(b, digits[i:]...)
}
