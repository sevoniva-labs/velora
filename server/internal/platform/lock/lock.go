package lock

import (
	"context"
	"errors"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
)

var ErrBusy = errors.New("lock already held")
var ErrUnavailable = errors.New("distributed lock unavailable")

type Manager struct{ cache cache.Cache }

func New(c cache.Cache) *Manager { return &Manager{cache: c} }

// Acquire returns a release function. Redis-backed cache makes it
// cluster-wide; memory cache is intentionally process-local.
func (m *Manager) Acquire(ctx context.Context, key string, ttl time.Duration) (func(context.Context) error, error) {
	if m.cache == nil || m.cache.Provider() == "disabled" {
		return nil, ErrUnavailable
	}
	token, err := cache.RandomToken(24)
	if err != nil {
		return nil, err
	}
	ok, err := m.cache.SetNX(ctx, "lock:"+key, token, ttl)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrBusy
	}
	return func(ctx context.Context) error {
		_, err := m.cache.CompareAndDelete(ctx, "lock:"+key, token)
		return err
	}, nil
}
