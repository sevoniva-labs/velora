package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
)

// Limiter uses cache atomic increment when available. Redis therefore gives a
// cluster-wide fixed-window limiter; memory is process-local.
type Limiter struct {
	cache cache.Cache
	mu    sync.Mutex
	local map[string]entry
}

const maxLocalEntries = 10_000

type entry struct {
	count int
	until time.Time
}

func New(c cache.Cache) *Limiter {
	return &Limiter{cache: c, local: map[string]entry{}}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	if l.cache != nil && l.cache.Provider() != "disabled" {
		n, err := l.cache.Increment(ctx, "ratelimit:"+key, window)
		if err == nil {
			return n <= int64(limit), nil
		}
		// Do not fail-open silently on a Redis outage in financial profile; the
		// caller can decide whether to convert this error into 503.
		if l.cache.Provider() == "redis" {
			return false, fmt.Errorf("distributed rate limit: %w", err)
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.local) >= maxLocalEntries {
		var oldestKey string
		var oldestUntil time.Time
		for k, v := range l.local {
			if now.After(v.until) {
				delete(l.local, k)
				continue
			}
			if oldestKey == "" || v.until.Before(oldestUntil) {
				oldestKey, oldestUntil = k, v.until
			}
		}
		if len(l.local) >= maxLocalEntries && oldestKey != "" {
			delete(l.local, oldestKey)
		}
	}
	e := l.local[key]
	if now.After(e.until) {
		e = entry{until: now.Add(window)}
	}
	if e.count >= limit {
		l.local[key] = e
		return false, nil
	}
	e.count++
	l.local[key] = e
	return true, nil
}
