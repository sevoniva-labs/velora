// Package ratelimit 提供分布式限流（Redis 固定窗口 + 内存降级）。
//
// Phase C3：替换 httpserver 的进程内限流为 Redis 实现，支持多实例统一限流；
// REDIS_URL 未配置时自动降级为单机内存实现（开发/演示环境可用）。
package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter 为限流器接口。
type Limiter interface {
	// Allow 判断 key 在窗口内是否允许（allowed 为 true 表示放行）。
	Allow(ctx context.Context, key string) (allowed bool, remaining int64, err error)
	// Reset 清除 key 的计数（解锁等场景）。
	Reset(ctx context.Context, key string) error
}

// Config 限流配置。
type Config struct {
	Limit  int           // 窗口内最大次数
	Window time.Duration // 窗口时长
}

// New 创建限流器。redisClient 为 nil 时返回内存实现。
func New(redisClient *redis.Client, cfg Config) Limiter {
	if redisClient != nil {
		return &redisLimiter{client: redisClient, cfg: cfg}
	}
	return newMemoryLimiter(cfg)
}

// --- Redis 实现（固定窗口，INCR + EXPIRE，原子） ---

type redisLimiter struct {
	client *redis.Client
	cfg    Config
}

func (l *redisLimiter) Allow(ctx context.Context, key string) (bool, int64, error) {
	redisKey := "rl:" + key
	// Lua 脚本保证原子性：计数 +1，首次设置过期；返回新计数。
	script := `
local c = redis.call('INCR', KEYS[1])
if c == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return c
`
	n, err := l.client.Eval(ctx, script, []string{redisKey}, l.cfg.Window.Milliseconds()).Int64()
	if err != nil {
		// Redis 故障：放行（fail-open，保证可用性优先；生产可改 fail-close）
		return true, 0, fmt.Errorf("限流器 Redis 调用失败（fail-open）: %w", err)
	}
	remaining := int64(l.cfg.Limit) - n
	if remaining < 0 {
		remaining = 0
	}
	return n <= int64(l.cfg.Limit), remaining, nil
}

func (l *redisLimiter) Reset(ctx context.Context, key string) error {
	return l.client.Del(ctx, "rl:"+key).Err()
}

// --- 内存实现（降级，单实例） ---

type memEntry struct {
	count int
	start time.Time
}

type memoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]*memEntry
	cfg     Config
}

func newMemoryLimiter(cfg Config) *memoryLimiter {
	l := &memoryLimiter{buckets: map[string]*memEntry{}, cfg: cfg}
	go l.sweeper()
	return l
}

func (l *memoryLimiter) sweeper() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, e := range l.buckets {
			if now.Sub(e.start) > l.cfg.Window {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *memoryLimiter) Allow(_ context.Context, key string) (bool, int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.buckets[key]
	if !ok || time.Since(e.start) > l.cfg.Window {
		e = &memEntry{start: time.Now()}
		l.buckets[key] = e
	}
	e.count++
	remaining := int64(l.cfg.Limit - e.count)
	if remaining < 0 {
		remaining = 0
	}
	return e.count <= l.cfg.Limit, remaining, nil
}

func (l *memoryLimiter) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
	return nil
}
