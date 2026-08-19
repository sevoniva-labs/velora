// Package metrics 提供轻量 Prometheus 文本格式指标（零第三方依赖）。
//
// 设计约束：
//   - 所有领域包（auth / application / audit / mail / todo / oidcprovider）通过
//     metrics.Emit / metrics.Observe 埋点，不 import httpserver，避免包循环。
//   - httpserver 的 /metrics 端点代理到本包的 Handler()，并注入 DB 池状态与运行时指标。
//   - 文本格式手写输出（Prometheus exposition format 0.0.4），不引入 SDK，
//     保持 server 二进制与 go.mod 精简。
package metrics

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

// 计数器：名称 → 计数。名称遵循 velora_<domain>_<event>_total。
type counter struct {
	name  string
	help  string
	value int64
}

// 直方图（延迟毫秒）：预定义桶，名称遵循 velora_<domain>_<event>_milliseconds。
type histogram struct {
	name    string
	help    string
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
}

var (
	mu         sync.RWMutex
	counters   = map[string]*counter{}
	histograms = map[string]*histogram{}
	started    = time.Now()
)

// 默认延迟桶（毫秒）：5/10/25/50/100/250/500/1000/2500/5000/10000/+Inf
var defaultBuckets = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

// RegisterCounter 注册计数器（幂等：已存在则忽略，保留首次 help）。
func RegisterCounter(name, help string) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := counters[name]; !ok {
		counters[name] = &counter{name: name, help: help}
	}
}

// RegisterHistogram 注册直方图（幂等）。
func RegisterHistogram(name, help string) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := histograms[name]; !ok {
		histograms[name] = &histogram{
			name:    name,
			help:    help,
			buckets: append([]float64(nil), defaultBuckets...),
			counts:  make([]int64, len(defaultBuckets)),
		}
	}
}

// Emit 对计数器 +1。
func Emit(name string) {
	mu.Lock()
	defer mu.Unlock()
	if c, ok := counters[name]; ok {
		c.value++
	}
}

// Observe 记录直方图观测值（毫秒）。
func Observe(name string, value float64) {
	mu.Lock()
	defer mu.Unlock()
	h, ok := histograms[name]
	if !ok {
		return
	}
	h.count++
	h.sum += value
	for i, b := range h.buckets {
		if value <= b {
			h.counts[i]++
		}
	}
}

// --- 常用指标注册（启动时调用） ---

// RegisterDefault 注册全系统默认指标。
func RegisterDefault() {
	RegisterCounter("velora_auth_login_success_total", "账号密码登录成功次数")
	RegisterCounter("velora_auth_login_failure_total", "账号密码登录失败次数")
	RegisterCounter("velora_auth_oidc_callback_total", "OIDC 授权码回调次数（含成功/失败）")
	RegisterCounter("velora_app_launch_total", "应用启动（Launch）次数")
	RegisterCounter("velora_app_sync_total", "Casdoor 应用同步次数")
	RegisterCounter("velora_audit_write_failure_total", "审计日志写入失败次数")
	RegisterCounter("velora_mail_sync_total", "邮件同步轮次")
	RegisterCounter("velora_mail_sync_failure_total", "邮件同步失败轮次")
	RegisterCounter("velora_oidc_authorize_total", "OIDC authorize 请求次数")
	RegisterCounter("velora_oidc_authorize_failure_total", "OIDC authorize 失败次数")
	RegisterCounter("velora_oidc_token_total", "OIDC token 签发次数")
	RegisterCounter("velora_oidc_token_failure_total", "OIDC token 签发失败次数")
	RegisterCounter("velora_todo_upsert_total", "待办推送（upsert）次数")
	RegisterCounter("velora_todo_done_total", "待办完成次数")
	RegisterHistogram("velora_http_request_duration_milliseconds", "HTTP 请求耗时（按 method/path）")
	RegisterHistogram("velora_oidc_token_duration_milliseconds", "OIDC token 签发耗时")
	RegisterHistogram("velora_mail_sync_duration_milliseconds", "邮件同步耗时")
}

// Render 输出 Prometheus 文本（contentType: text/plain; version=0.0.4）。
// dbStats 可为 nil；runtime 指标始终输出。
func Render(dbStats func() (open, inUse, idle int64, ok bool)) []byte {
	mu.RLock()
	defer mu.RUnlock()

	var sb []byte
	sb = append(sb, "# HELP velora_up Whether Velora is up.\n# TYPE velora_up gauge\nvelora_up 1\n"...)
	sb = append(sb, "# HELP velora_uptime_seconds Process uptime in seconds.\n# TYPE velora_uptime_seconds gauge\nvelora_uptime_seconds "...)
	sb = appendInt(sb, int64(time.Since(started).Seconds()))
	sb = append(sb, '\n')

	// 运行时指标
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	writeGauge(&sb, "velora_go_goroutines", "Number of goroutines that currently exist.", int64(runtime.NumGoroutine()))
	writeGauge(&sb, "velora_go_heap_alloc_bytes", "Bytes of allocated heap objects.", int64(ms.HeapAlloc))
	writeGauge(&sb, "velora_go_gc_cycles_total", "Total number of completed GC cycles.", int64(ms.NumGC))

	// 数据库连接池
	if dbStats != nil {
		if open, inUse, idle, ok := dbStats(); ok {
			writeGauge(&sb, "velora_db_connections_open", "Number of open database connections.", open)
			writeGauge(&sb, "velora_db_connections_in_use", "Number of database connections in use.", inUse)
			writeGauge(&sb, "velora_db_connections_idle", "Number of idle database connections.", idle)
		}
	}

	// 计数器（按名称排序保证输出稳定）
	names := make([]string, 0, len(counters))
	for n := range counters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := counters[n]
		sb = append(sb, "# HELP "...)
		sb = append(sb, c.name...)
		sb = append(sb, ' ')
		sb = append(sb, c.help...)
		sb = append(sb, "\n# TYPE "...)
		sb = append(sb, c.name...)
		sb = append(sb, " counter\n"...)
		sb = append(sb, c.name...)
		sb = append(sb, ' ')
		sb = appendInt(sb, c.value)
		sb = append(sb, '\n')
	}

	// 直方图
	hnames := make([]string, 0, len(histograms))
	for n := range histograms {
		hnames = append(hnames, n)
	}
	sort.Strings(hnames)
	for _, n := range hnames {
		h := histograms[n]
		sb = append(sb, "# HELP "...)
		sb = append(sb, h.name...)
		sb = append(sb, ' ')
		sb = append(sb, h.help...)
		sb = append(sb, "\n# TYPE "...)
		sb = append(sb, h.name...)
		sb = append(sb, " histogram\n"...)
		for i, b := range h.buckets {
			sb = append(sb, h.name...)
			sb = append(sb, "_bucket{le=\""...)
			sb = appendFloat(sb, b)
			sb = append(sb, "\"} "...)
			sb = appendInt(sb, h.counts[i])
			sb = append(sb, '\n')
		}
		sb = append(sb, h.name...)
		sb = append(sb, "_bucket{le=\"+Inf\"} "...)
		sb = appendInt(sb, h.count)
		sb = append(sb, '\n')
		sb = append(sb, h.name...)
		sb = append(sb, "_sum "...)
		sb = appendFloat(sb, h.sum)
		sb = append(sb, '\n')
		sb = append(sb, h.name...)
		sb = append(sb, "_count "...)
		sb = appendInt(sb, h.count)
		sb = append(sb, '\n')
	}
	return sb
}

func writeGauge(sb *[]byte, name, help string, v int64) {
	*sb = append(*sb, "# HELP "...)
	*sb = append(*sb, name...)
	*sb = append(*sb, ' ')
	*sb = append(*sb, help...)
	*sb = append(*sb, "\n# TYPE "...)
	*sb = append(*sb, name...)
	*sb = append(*sb, " gauge\n"...)
	*sb = append(*sb, name...)
	*sb = append(*sb, ' ')
	*sb = appendInt(*sb, v)
	*sb = append(*sb, '\n')
}

func appendInt(b []byte, v int64) []byte {
	return append(b, fmt.Sprintf("%d", v)...)
}

func appendFloat(b []byte, v float64) []byte {
	return append(b, fmt.Sprintf("%g", v)...)
}
