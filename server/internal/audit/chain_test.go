package audit

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestAuditService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditLog{}))
	return NewService(db)
}

// recordRaw 直接写一条审计（绕过 Record 的 gin.Context 依赖），模拟真实记录。
func recordRaw(t *testing.T, s *Service, e Entry) {
	t.Helper()
	prev := s.lastHash(context.Background())
	now := time.Now().UTC()
	log := AuditLog{
		Operator:   e.Operator,
		Action:     e.Action,
		Resource:   e.Resource,
		ResourceID: e.ResourceID,
		IP:         "127.0.0.1",
		UserAgent:  "test",
		RequestID:  "req-1",
		Detail:     e.Detail,
		PrevHash:   prev,
		CreatedAt:  now,
	}
	log.Hash = chainHash(prev, e.Action, e.Resource, e.ResourceID, e.Operator, log.IP, e.Detail, now)
	require.NoError(t, s.db.Create(&log).Error)
}

func TestChainHashStability(t *testing.T) {
	now := time.Now().UTC()
	// 相同输入 → 相同哈希（确定性）
	h1 := chainHash("prev", "LOGIN", "session", "rid", "admin", "1.2.3.4", "detail", now)
	h2 := chainHash("prev", "LOGIN", "session", "rid", "admin", "1.2.3.4", "detail", now)
	assert.Equal(t, h1, h2)
	// 任一字段变化 → 哈希变化
	h3 := chainHash("prev", "LOGIN", "session", "rid", "admin", "1.2.3.4", "other", now)
	assert.NotEqual(t, h1, h3)
}

func TestVerifyChainOK(t *testing.T) {
	s := newTestAuditService(t)
	recordRaw(t, s, Entry{Operator: "u1", Action: ActionLogin, Resource: "session"})
	recordRaw(t, s, Entry{Operator: "u2", Action: ActionAppCreate, Resource: "applications"})
	recordRaw(t, s, Entry{Operator: "u3", Action: ActionOIDCAuthorize, Resource: "oidc"})

	ok, badID, err := s.VerifyChain(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, uint64(0), badID)
}

func TestVerifyChainDetectsTamper(t *testing.T) {
	s := newTestAuditService(t)
	recordRaw(t, s, Entry{Operator: "u1", Action: ActionLogin, Resource: "session"})
	recordRaw(t, s, Entry{Operator: "u2", Action: ActionAppCreate, Resource: "applications"})
	recordRaw(t, s, Entry{Operator: "u3", Action: ActionOIDCAuthorize, Resource: "oidc"})

	// 篡改中间记录（u2 的 action）
	require.NoError(t, s.db.Model(&AuditLog{}).Where("action = ?", ActionAppCreate).Update("action", ActionAppDelete).Error)

	ok, badID, err := s.VerifyChain(context.Background(), 0)
	require.NoError(t, err, "篡改检测不应是系统错误")
	assert.False(t, ok, "篡改应被检测")
	assert.Equal(t, uint64(2), badID, "不一致应定位到第 2 条")
}

// TestConcurrentRecordChainIntegrity 并发写入审计记录后链仍完整（无分叉）。
func TestConcurrentRecordChainIntegrity(t *testing.T) {
	db := newTestAuditService(t).db
	svc := NewService(db)

	const n = 50
	var wg sync.WaitGroup
	errsCh := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := svc.withChainLock(context.Background(), func(tx *gorm.DB) error {
				prev := lastHashTx(tx)
				rec := AuditLog{
					Operator:  fmt.Sprintf("u-%d", i%5),
					Action:    "CONCURRENT_TEST",
					Resource:  "chain",
					Detail:    "c",
					CreatedAt: time.Now(),
					PrevHash:  prev,
				}
				rec.Hash = chainHash(prev, rec.Action, rec.Resource, rec.ResourceID, rec.Operator, rec.IP, rec.Detail, rec.CreatedAt)
				return tx.Create(&rec).Error
			})
			errsCh <- err
		}(i)
	}
	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		require.NoError(t, err)
	}

	ok, badID, err := svc.VerifyChain(context.Background(), 0)
	require.NoError(t, err)
	assert.True(t, ok, "并发写入后链应完整；断裂记录 id=%d", badID)
}
