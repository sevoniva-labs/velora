package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLimiter(t *testing.T) {
	l := New(nil, Config{Limit: 3, Window: time.Minute})
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, _, err := l.Allow(ctx, "ip-1")
		require.NoError(t, err)
		assert.True(t, ok)
	}
	// 第 4 次超限
	ok, remaining, err := l.Allow(ctx, "ip-1")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, int64(0), remaining)
	// 其他 key 不受影响
	ok, _, err = l.Allow(ctx, "ip-2")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMemoryLimiterReset(t *testing.T) {
	l := New(nil, Config{Limit: 2, Window: time.Minute})
	ctx := context.Background()
	_, _, _ = l.Allow(ctx, "k")
	_, _, _ = l.Allow(ctx, "k")
	ok, _, _ := l.Allow(ctx, "k")
	assert.False(t, ok)
	require.NoError(t, l.Reset(ctx, "k"))
	ok, _, err := l.Allow(ctx, "k")
	require.NoError(t, err)
	assert.True(t, ok)
}
