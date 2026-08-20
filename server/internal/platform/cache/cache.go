package cache

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/tlsx"
)

var ErrMiss = errors.New("cache miss")
var ErrNotSupported = errors.New("operation not supported by cache provider")

// Cache is intentionally slightly richer than a plain KV store because
// distributed rate limiting and locking need atomic primitives.
type Cache interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, string, time.Duration) error
	Delete(context.Context, string) error
	Increment(context.Context, string, time.Duration) (int64, error)
	SetNX(context.Context, string, string, time.Duration) (bool, error)
	CompareAndDelete(context.Context, string, string) (bool, error)
	Ping(context.Context) error
	Close() error
	Provider() string
}

func New(cfg config.Cache) (Cache, error) {
	switch cfg.Provider {
	case "disabled":
		return noop{}, nil
	case "memory", "":
		return newMemory(cfg.Prefix), nil
	case "redis":
		return newRedis(cfg)
	default:
		return nil, errors.New("unsupported cache provider")
	}
}

type noop struct{}

func (noop) Get(context.Context, string) (string, error)              { return "", ErrMiss }
func (noop) Set(context.Context, string, string, time.Duration) error { return nil }
func (noop) Delete(context.Context, string) error                     { return nil }
func (noop) Increment(context.Context, string, time.Duration) (int64, error) {
	return 0, ErrNotSupported
}
func (noop) SetNX(context.Context, string, string, time.Duration) (bool, error) {
	return false, ErrNotSupported
}
func (noop) CompareAndDelete(context.Context, string, string) (bool, error) {
	return false, ErrNotSupported
}
func (noop) Ping(context.Context) error { return nil }
func (noop) Close() error               { return nil }
func (noop) Provider() string           { return "disabled" }

type item struct {
	value   string
	counter int64
	expires time.Time
}
type memory struct {
	mu     sync.RWMutex
	prefix string
	data   map[string]item
}

func newMemory(prefix string) *memory  { return &memory{prefix: prefix, data: map[string]item{}} }
func (m *memory) full(k string) string { return m.prefix + k }
func (m *memory) expired(v item) bool  { return !v.expires.IsZero() && time.Now().After(v.expires) }

func (m *memory) Get(_ context.Context, k string) (string, error) {
	m.mu.RLock()
	v, ok := m.data[m.full(k)]
	m.mu.RUnlock()
	if !ok || m.expired(v) {
		return "", ErrMiss
	}
	return v.value, nil
}
func (m *memory) Set(_ context.Context, k, v string, ttl time.Duration) error {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.data[m.full(k)] = item{value: v, expires: exp}
	m.mu.Unlock()
	return nil
}
func (m *memory) Delete(_ context.Context, k string) error {
	m.mu.Lock()
	delete(m.data, m.full(k))
	m.mu.Unlock()
	return nil
}
func (m *memory) Increment(_ context.Context, k string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.full(k)
	v, ok := m.data[key]
	if !ok || m.expired(v) {
		v = item{}
		if ttl > 0 {
			v.expires = time.Now().Add(ttl)
		}
	}
	v.counter++
	v.value = ""
	m.data[key] = v
	return v.counter, nil
}
func (m *memory) SetNX(_ context.Context, k, v string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.full(k)
	if old, ok := m.data[key]; ok && !m.expired(old) {
		return false, nil
	}
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.data[key] = item{value: v, expires: exp}
	return true, nil
}
func (m *memory) CompareAndDelete(_ context.Context, k, expected string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.full(k)
	v, ok := m.data[key]
	if !ok || m.expired(v) {
		delete(m.data, key)
		return false, nil
	}
	if subtle.ConstantTimeCompare([]byte(v.value), []byte(expected)) != 1 {
		return false, nil
	}
	delete(m.data, key)
	return true, nil
}
func (m *memory) Ping(context.Context) error { return nil }
func (m *memory) Close() error               { return nil }
func (m *memory) Provider() string           { return "memory" }

type redisCache struct {
	client redis.UniversalClient
	prefix string
}

func newRedis(cfg config.Cache) (*redisCache, error) {
	addrs := append([]string(nil), cfg.Addresses...)
	if len(addrs) == 0 {
		return nil, errors.New("redis addresses required")
	}
	tlsCfg, err := tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled: cfg.TLS, CAFile: cfg.TLSCAFile, CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, ServerName: cfg.TLSServerName,
	})
	if err != nil {
		return nil, err
	}
	opts := &redis.UniversalOptions{
		Addrs: addrs, MasterName: cfg.MasterName, Username: cfg.Username, Password: cfg.Password,
		DB: cfg.DB, TLSConfig: tlsCfg,
	}
	return &redisCache{client: redis.NewUniversalClient(opts), prefix: cfg.Prefix}, nil
}

func (r *redisCache) full(k string) string { return r.prefix + k }
func (r *redisCache) Get(ctx context.Context, k string) (string, error) {
	v, e := r.client.Get(ctx, r.full(k)).Result()
	if e == redis.Nil {
		return "", ErrMiss
	}
	return v, e
}
func (r *redisCache) Set(ctx context.Context, k, v string, ttl time.Duration) error {
	return r.client.Set(ctx, r.full(k), v, ttl).Err()
}
func (r *redisCache) Delete(ctx context.Context, k string) error {
	return r.client.Del(ctx, r.full(k)).Err()
}

var incrementScript = redis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 and tonumber(ARGV[1]) > 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return n
`)

func (r *redisCache) Increment(ctx context.Context, k string, ttl time.Duration) (int64, error) {
	return incrementScript.Run(ctx, r.client, []string{r.full(k)}, ttl.Milliseconds()).Int64()
}
func (r *redisCache) SetNX(ctx context.Context, k, v string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, r.full(k), v, ttl).Result()
}

var compareDeleteScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (r *redisCache) CompareAndDelete(ctx context.Context, k, expected string) (bool, error) {
	n, err := compareDeleteScript.Run(ctx, r.client, []string{r.full(k)}, expected).Int64()
	return n == 1, err
}
func (r *redisCache) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }
func (r *redisCache) Close() error                   { return r.client.Close() }
func (r *redisCache) Provider() string               { return "redis" }

// RandomToken is shared by lock/idempotency helpers without exposing Redis.
func RandomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
