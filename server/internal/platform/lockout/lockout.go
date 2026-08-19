// Package lockout 提供账户锁定（防暴力破解）。
//
// Phase C3：登录失败计数（Redis 分布式）达到阈值后锁定账户；
// 锁定窗口内即使密码正确也拒绝；登录成功或解锁后清零。
package lockout

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config 账户锁定策略。
type Config struct {
	// MaxFailures 窗口内最大失败次数（默认 5）。
	MaxFailures int
	// Window 失败计数窗口（默认 15 分钟）。
	Window time.Duration
	// LockDuration 锁定时长（默认 15 分钟）。
	LockDuration time.Duration
}

// DefaultConfig 返回默认策略：5 次失败 / 15 分钟窗口 → 锁定 15 分钟。
func DefaultConfig() Config {
	return Config{MaxFailures: 5, Window: 15 * time.Minute, LockDuration: 15 * time.Minute}
}

// Manager 账户锁定管理器。
type Manager struct {
	client *redis.Client // nil 时降级内存
	cfg    Config

	mu      map[string]*memLock
	muGuard chan struct{}
}

type memLock struct {
	failures int
	windowStart time.Time
	lockedUntil time.Time
}

// New 创建锁定管理器。
func New(client *redis.Client, cfg Config) *Manager {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.Window <= 0 {
		cfg.Window = 15 * time.Minute
	}
	if cfg.LockDuration <= 0 {
		cfg.LockDuration = 15 * time.Minute
	}
	return &Manager{client: client, cfg: cfg, mu: map[string]*memLock{}, muGuard: make(chan struct{}, 1)}
}

// IsLocked 判断账户是否处于锁定状态。
func (m *Manager) IsLocked(ctx context.Context, username string) (bool, time.Duration, error) {
	if m.client != nil {
		return m.redisIsLocked(ctx, username)
	}
	m.muGuard <- struct{}{}
	defer func() { <-m.muGuard }()
	e, ok := m.mu[username]
	if !ok {
		return false, 0, nil
	}
	if time.Now().Before(e.lockedUntil) {
		return true, time.Until(e.lockedUntil), nil
	}
	return false, 0, nil
}

// RecordFailure 记录一次登录失败（达到阈值 → 锁定）。
func (m *Manager) RecordFailure(ctx context.Context, username string) (locked bool, err error) {
	if m.client != nil {
		return m.redisRecordFailure(ctx, username)
	}
	m.muGuard <- struct{}{}
	defer func() { <-m.muGuard }()
	now := time.Now()
	e, ok := m.mu[username]
	if !ok || now.Sub(e.windowStart) > m.cfg.Window {
		e = &memLock{windowStart: now}
		m.mu[username] = e
	}
	e.failures++
	if e.failures >= m.cfg.MaxFailures {
		e.lockedUntil = now.Add(m.cfg.LockDuration)
		e.failures = 0 // 锁定期间不再累计
		return true, nil
	}
	return false, nil
}

// RecordSuccess 登录成功：清零失败计数与锁定。
func (m *Manager) RecordSuccess(ctx context.Context, username string) error {
	if m.client != nil {
		return m.client.Del(ctx, "lock:"+username, "lockfail:"+username).Err()
	}
	m.muGuard <- struct{}{}
	defer func() { <-m.muGuard }()
	delete(m.mu, username)
	return nil
}

// Unlock 管理员手动解锁。
func (m *Manager) Unlock(ctx context.Context, username string) error {
	return m.RecordSuccess(ctx, username)
}

// --- Redis 实现 ---

func (m *Manager) redisIsLocked(ctx context.Context, username string) (bool, time.Duration, error) {
	ttl, err := m.client.TTL(ctx, "lock:"+username).Result()
	if err != nil {
		return false, 0, fmt.Errorf("查询锁定状态失败: %w", err)
	}
	if ttl > 0 {
		return true, ttl, nil
	}
	return false, 0, nil
}

func (m *Manager) redisRecordFailure(ctx context.Context, username string) (bool, error) {
	key := "lockfail:" + username
	// Lua：计数 +1，首次设置窗口过期；超过阈值则设置锁定 key。
	script := `
local c = redis.call('INCR', KEYS[1])
if c == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
if c >= tonumber(ARGV[2]) then
  redis.call('SET', KEYS[2], '1', 'PX', ARGV[3])
  redis.call('DEL', KEYS[1])
  return 1
end
return 0
`
	locked, err := m.client.Eval(ctx, script,
		[]string{key, "lock:" + username},
		m.cfg.Window.Milliseconds(), m.cfg.MaxFailures, m.cfg.LockDuration.Milliseconds()).Int64()
	if err != nil {
		return false, fmt.Errorf("记录失败计数失败: %w", err)
	}
	return locked == 1, nil
}
