package forwardauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

// newTestEnv 组装 SessionStore（内存 sqlite sessions 表）+ gin 路由。
func newTestEnv(t *testing.T) (*gin.Engine, *auth.SessionStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&auth.ServerSession{}))

	sessions, err := auth.NewSessionStore("0123456789abcdef0123456789abcdef", time.Hour, false, "")
	require.NoError(t, err)
	sessions.SetDB(db)

	h := NewHandler(sessions, "/login?redirect=")
	r := gin.New()
	h.Register(r.Group("/api/v1"))
	return r, sessions
}

func TestForwardAuthUnauthenticated(t *testing.T) {
	r, _ := newTestEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/forward-auth?next=https://legacy.example.com/dash", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/login?redirect=https%3A%2F%2Flegacy.example.com%2Fdash", w.Header().Get("Location"))
	assert.Empty(t, w.Header().Get("X-Velora-User"))
}

func TestForwardAuthAuthenticated(t *testing.T) {
	r, sessions := newTestEnv(t)
	// 登录态：构造会话并写入 cookie。
	user := &auth.CurrentUser{ID: "u-1", Username: "alice", Email: "alice@corp.com", Roles: []string{"velora_admin"}}
	sess := sessions.NewSession(user)
	token, err := sessions.EncodeWithMeta(sess, "test-agent", "10.0.0.8")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/forward-auth", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "alice", w.Header().Get("X-Velora-User"))
	assert.Equal(t, "alice@corp.com", w.Header().Get("X-Velora-Email"))
	assert.Equal(t, "velora_admin", w.Header().Get("X-Velora-Role"))
}

func TestForwardAuthRevokedSession(t *testing.T) {
	r, sessions := newTestEnv(t)
	user := &auth.CurrentUser{ID: "u-1", Username: "alice"}
	sess := sessions.NewSession(user)
	token, err := sessions.EncodeWithMeta(sess, "", "")
	require.NoError(t, err)
	require.NoError(t, sessions.Revoke(sess.SID))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/forward-auth", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "已吊销会话应拒绝")
	assert.Empty(t, w.Header().Get("X-Velora-User"))
}

func TestForwardAuthEvilNext(t *testing.T) {
	r, _ := newTestEnv(t)
	// 非 http(s) 的 next：不拼进 Location（拒绝协议相对/伪协议）。
	req := httptest.NewRequest(http.MethodGet, "/api/v1/forward-auth?next=javascript:alert(1)", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/login?redirect=", w.Header().Get("Location"), "非法 next 不应拼接")
}
