package serviceaccount

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestTokenService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&IntegrationToken{}))
	return NewService(db)
}

func TestCreateAndAuthenticate(t *testing.T) {
	s := newTestTokenService(t)
	ctx := context.Background()

	plain, err := s.Create(ctx, "工单系统", "admin", []string{ScopeTodoWrite}, nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(plain, "velora_"))

	rec, err := s.Authenticate(ctx, plain)
	require.NoError(t, err)
	assert.Equal(t, "工单系统", rec.Name)
	assert.True(t, rec.HasScope(ScopeTodoWrite))
	assert.False(t, rec.HasScope("other:scope"))

	// 明文不落库（库中只有哈希）
	var stored IntegrationToken
	require.NoError(t, s.db.First(&stored).Error)
	assert.NotEqual(t, plain, stored.TokenHash)
	assert.NotEmpty(t, stored.TokenHash)
}

func TestAuthenticateWrongToken(t *testing.T) {
	s := newTestTokenService(t)
	_, err := s.Authenticate(context.Background(), "velora_wrongtoken")
	assert.Error(t, err)
}

func TestRevoke(t *testing.T) {
	s := newTestTokenService(t)
	ctx := context.Background()
	plain, err := s.Create(ctx, "jenkins", "admin", []string{ScopeTodoWrite}, nil)
	require.NoError(t, err)

	var rec IntegrationToken
	require.NoError(t, s.db.Where("name = ?", "jenkins").First(&rec).Error)
	require.NoError(t, s.Revoke(ctx, rec.ID))

	_, err = s.Authenticate(ctx, plain)
	assert.Error(t, err, "吊销后应失效")

	// 重复吊销 → 错误
	assert.Error(t, s.Revoke(ctx, rec.ID))
}

func TestExpiredToken(t *testing.T) {
	s := newTestTokenService(t)
	ctx := context.Background()
	past := time.Now().Add(-time.Hour)
	plain, err := s.Create(ctx, "expired", "admin", []string{ScopeTodoWrite}, &past)
	require.NoError(t, err)
	_, err = s.Authenticate(ctx, plain)
	assert.Error(t, err, "过期令牌应拒绝")
}
