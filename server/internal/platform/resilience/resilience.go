package resilience

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

var ErrCircuitOpen = errors.New("circuit breaker open")
var ErrBulkheadFull = errors.New("bulkhead full")

type Circuit struct {
	mu        sync.Mutex
	failures  int
	openedAt  time.Time
	threshold int
	openFor   time.Duration
}

func NewCircuit(cfg config.Resilience) *Circuit {
	return &Circuit{threshold: cfg.CircuitFailureThreshold, openFor: cfg.CircuitOpenDuration}
}
func (c *Circuit) Execute(ctx context.Context, fn func(context.Context) error) error {
	c.mu.Lock()
	if !c.openedAt.IsZero() {
		if time.Since(c.openedAt) < c.openFor {
			c.mu.Unlock()
			return ErrCircuitOpen
		}
		c.openedAt = time.Time{} // half-open probe
	}
	c.mu.Unlock()

	err := fn(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.failures = 0
		c.openedAt = time.Time{}
		return nil
	}
	c.failures++
	if c.failures >= c.threshold {
		c.openedAt = time.Now()
		c.failures = 0
	}
	return err
}

type Bulkhead struct{ ch chan struct{} }

func NewBulkhead(n int) *Bulkhead { return &Bulkhead{ch: make(chan struct{}, n)} }
func (b *Bulkhead) Execute(ctx context.Context, fn func(context.Context) error) error {
	select {
	case b.ch <- struct{}{}:
		defer func() { <-b.ch }()
		return fn(ctx)
	default:
		return ErrBulkheadFull
	}
}

func Retry(ctx context.Context, max int, base time.Duration, fn func(context.Context) error) error {
	if max < 1 {
		max = 1
	}
	var last error
	for i := 0; i < max; i++ {
		if err := fn(ctx); err == nil {
			return nil
		} else {
			last = err
		}
		if i == max-1 {
			break
		}
		delay := base * time.Duration(1<<min(i, 6))
		jitter := randomJitter(maxDuration(delay/4, time.Millisecond))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay + jitter):
		}
	}
	return last
}

func randomJitter(maximum time.Duration) time.Duration {
	if maximum <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maximum)))
	if err != nil {
		return 0
	}
	return time.Duration(n.Int64())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
