package serviceaccount

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

// newTestHandler 组装带管理中间件的路由（用可注入的当前用户）。
// returns (router, service)。
func newTestHandler(t *testing.T, user *auth.CurrentUser) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&IntegrationToken{}))
	svc := NewService(db)
	h := NewHandler(svc, "velora_admin")
	r := gin.New()
	// 模拟 Auth 中间件：注入当前用户。
	r.Use(func(c *gin.Context) {
		if user != nil {
			auth.SetCurrentUser(c, user)
		}
	})
	// 管理员中间件：仅当用户是 admin 才放行。
	r.Use(func(c *gin.Context) {
		u, err := auth.RequireUser(c)
		if err != nil || !u.IsAdmin("velora_admin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "A03001"})
			return
		}
		c.Next()
	})
	h.Register(r.Group("/api/v1"))
	return r, svc
}

func TestTokenCreateListRevokeFlow(t *testing.T) {
	admin := &auth.CurrentUser{ID: "u-admin", Username: "admin", Roles: []string{"velora_admin"}}
	r, svc := newTestHandler(t, admin)

	// 创建
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integration-tokens", strings.NewReader(`{"name":"ci-system","scopes":["todo:write"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var created struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.True(t, strings.HasPrefix(created.Data.Token, "velora_"), "明文 token 仅创建响应返回")

	// 列表：不含明文，含创建者
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/integration-tokens", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	var listResp struct {
		Data []IntegrationToken `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data, 1)
	assert.Equal(t, "ci-system", listResp.Data[0].Name)
	assert.Equal(t, "admin", listResp.Data[0].CreatedBy)
	assert.False(t, listResp.Data[0].Revoked)
	assert.NotContains(t, w2.Body.String(), "velora_", "列表不得泄露明文 token")

	// 吊销
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/integration-tokens/1", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	require.Equal(t, http.StatusOK, w3.Code)

	// 吊销后 Bearer 鉴权失败（服务层闭环）
	_, err := svc.Authenticate(t.Context(), created.Data.Token)
	assert.Error(t, err, "吊销后令牌应失效")
}

func TestTokenRequiresAdmin(t *testing.T) {
	// 普通用户（非 admin）访问管理端点 → 403
	normal := &auth.CurrentUser{ID: "u-2", Username: "bob"}
	r, _ := newTestHandler(t, normal)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/integration-tokens", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestTokenCreateValidation(t *testing.T) {
	admin := &auth.CurrentUser{ID: "u-admin", Username: "admin", Roles: []string{"velora_admin"}}
	r, _ := newTestHandler(t, admin)

	// 缺名称
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integration-tokens", strings.NewReader(`{"scopes":["todo:write"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 缺 scope
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/integration-tokens", strings.NewReader(`{"name":"x"}`))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}
