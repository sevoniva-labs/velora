package privacy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
)

// newTestExportHandler 组装导出端点（AdminRequired 由测试中间件模拟）。
func newTestExportHandler(t *testing.T, user *auth.CurrentUser) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE application_favorites (user_id TEXT, application_id INTEGER, created_at DATETIME, PRIMARY KEY(user_id, application_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE application_visits (user_id TEXT, application_id INTEGER, visit_count INTEGER, last_visited_at DATETIME, PRIMARY KEY(user_id, application_id))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE todos (id INTEGER PRIMARY KEY, user_id TEXT, title TEXT, kind TEXT, source_system TEXT, source_id TEXT, priority TEXT, status TEXT, due_at DATETIME, created_at DATETIME)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE mail_messages (id INTEGER PRIMARY KEY, user_id TEXT, folder TEXT, subject TEXT, from_address TEXT, from_name TEXT, to_addresses TEXT, received_at DATETIME, is_read INTEGER, is_starred INTEGER, has_attachment INTEGER)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE audit_logs (id INTEGER PRIMARY KEY, operator TEXT, action TEXT, resource TEXT, resource_id TEXT, ip TEXT, detail TEXT, prev_hash TEXT, hash TEXT, created_at DATETIME)`).Error)

	svc := NewService(db)
	auditSvc := audit.NewService(db)
	h := NewHandler(svc, auditSvc, "velora_admin")
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != nil {
			auth.SetCurrentUser(c, user)
		}
	})
	r.Use(func(c *gin.Context) {
		u, err := auth.RequireUser(c)
		if err != nil || !u.IsAdmin("velora_admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "A03001"})
			return
		}
		c.Next()
	})
	h.Register(r.Group("/api/v1"))
	return r
}

func TestExportDownload(t *testing.T) {
	admin := &auth.CurrentUser{ID: "u-admin", Username: "admin", Roles: []string{"velora_admin"}}
	r := newTestExportHandler(t, admin)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/u-target/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), `attachment; filename="velora-user-u-target-`)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var out Export
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "u-target", out.UserID)
	assert.NotEmpty(t, out.GeneratedAt)
}

func TestExportRequiresAdmin(t *testing.T) {
	normal := &auth.CurrentUser{ID: "u-2", Username: "bob"}
	r := newTestExportHandler(t, normal)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/u-target/export", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}
