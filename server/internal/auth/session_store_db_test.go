package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newTestServerStore 构建启用 DB 服务端会话的 SessionStore。
func newTestServerStore(t *testing.T) *SessionStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ServerSession{}))
	s, err := NewSessionStore(strings.Repeat("s", 32), time.Hour, false, "")
	require.NoError(t, err)
	s.SetDB(db)
	return s
}

func TestServerSessionPersistAndDecode(t *testing.T) {
	s := newTestServerStore(t)
	user := &CurrentUser{ID: "u-1", Username: "carson", Email: "c@example.com", Roles: []string{"velora_admin"}}
	session := s.NewSession(user)
	token, err := s.EncodeWithMeta(session, "Mozilla/5.0", "127.0.0.1")
	require.NoError(t, err)
	assert.NotEmpty(t, session.SID, "服务端会话应生成 SID")

	decoded, err := s.Decode(token)
	require.NoError(t, err)
	assert.Equal(t, "u-1", decoded.UserID)
	assert.Equal(t, "carson", decoded.Username)

	// DB 记录带 UA/IP
	var rec ServerSession
	require.NoError(t, s.db.Where("session_id = ?", session.SID).First(&rec).Error)
	assert.Equal(t, "Mozilla/5.0", rec.UserAgent)
	assert.Equal(t, "127.0.0.1", rec.IP)
	assert.Equal(t, "carson", rec.Username)
}

func TestServerSessionRevoke(t *testing.T) {
	s := newTestServerStore(t)
	user := &CurrentUser{ID: "u-1", Username: "carson"}
	session := s.NewSession(user)
	token, err := s.Encode(session)
	require.NoError(t, err)

	// 吊销前可解码
	_, err = s.Decode(token)
	require.NoError(t, err)

	// 吊销后拒绝
	require.NoError(t, s.Revoke(session.SID))
	_, err = s.Decode(token)
	require.Error(t, err)
}

func TestServerSessionLegacyCompatibility(t *testing.T) {
	s := newTestServerStore(t)
	// 模拟旧式无状态会话：无 SID，直接编码（绕过 persist）
	user := &CurrentUser{ID: "u-1", Username: "carson"}
	session := s.NewSession(user)
	session.SID = "" // 旧格式无 SID
	payload, _ := json.Marshal(session)
	sig := s.sign(payload)
	legacyToken := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)

	decoded, err := s.Decode(legacyToken)
	require.NoError(t, err, "旧式无状态会话应兼容")
	assert.Equal(t, "u-1", decoded.UserID)
}

func TestServerSessionRevokeAllForUser(t *testing.T) {
	s := newTestServerStore(t)
	user := &CurrentUser{ID: "u-1", Username: "carson"}
	s1 := s.NewSession(user)
	t1, err := s.Encode(s1)
	require.NoError(t, err)
	s2 := s.NewSession(user)
	t2, err := s.Encode(s2)
	require.NoError(t, err)

	// 吊销全部
	require.NoError(t, s.RevokeAllForUser("u-1"))
	_, err = s.Decode(t1)
	require.Error(t, err)
	_, err = s.Decode(t2)
	require.Error(t, err)

	// 设备列表
	list, err := s.ListForUser("u-1")
	require.NoError(t, err)
	assert.Len(t, list, 2)
	for _, r := range list {
		assert.NotNil(t, r.RevokedAt)
	}
}
