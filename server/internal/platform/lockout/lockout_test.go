package lockout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLockoutThreshold(t *testing.T) {
	m := New(nil, Config{MaxFailures: 3, Window: time.Minute, LockDuration: time.Minute})
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		locked, err := m.RecordFailure(ctx, "alice")
		require.NoError(t, err)
		assert.False(t, locked)
	}
	// 第 3 次失败 → 锁定
	locked, err := m.RecordFailure(ctx, "alice")
	require.NoError(t, err)
	assert.True(t, locked)

	// 锁定期间 IsLocked = true
	isLocked, ttl, err := m.IsLocked(ctx, "alice")
	require.NoError(t, err)
	assert.True(t, isLocked)
	assert.Greater(t, ttl, time.Duration(0))
}

func TestMemoryLockoutResetOnSuccess(t *testing.T) {
	m := New(nil, Config{MaxFailures: 3, Window: time.Minute, LockDuration: time.Minute})
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, _ = m.RecordFailure(ctx, "bob")
	}
	require.NoError(t, m.RecordSuccess(ctx, "bob"))
	isLocked, _, err := m.IsLocked(ctx, "bob")
	require.NoError(t, err)
	assert.False(t, isLocked, "成功后应清零")
}

func TestMemoryLockoutWindowExpiry(t *testing.T) {
	m := New(nil, Config{MaxFailures: 2, Window: 50 * time.Millisecond, LockDuration: time.Minute})
	ctx := context.Background()
	_, _ = m.RecordFailure(ctx, "carol")
	time.Sleep(60 * time.Millisecond)
	// 窗口过期：计数应重置，第 2 次失败不锁定（若窗口未重置则第 2 次就锁）
	locked, err := m.RecordFailure(ctx, "carol")
	require.NoError(t, err)
	assert.False(t, locked, "窗口过期后计数应重置")
}

func TestMemoryLockoutUnlock(t *testing.T) {
	m := New(nil, Config{MaxFailures: 2, Window: time.Minute, LockDuration: time.Minute})
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, _ = m.RecordFailure(ctx, "dave")
	}
	require.NoError(t, m.Unlock(ctx, "dave"))
	isLocked, _, err := m.IsLocked(ctx, "dave")
	require.NoError(t, err)
	assert.False(t, isLocked)
}
