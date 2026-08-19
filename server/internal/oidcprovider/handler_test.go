// Phase B8 验收测试：OIDC Provider 端到端安全行为（handler 层）。
package oidcprovider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

// sqliteOpen 打开独立内存库（按测试名隔离，避免共享状态）。
func sqliteOpen(name string) string {
	return "file:" + name + "?mode=memory&cache=shared"
}

// s256Challenge 计算 PKCE S256 challenge。
func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// oidcTestEnv 组装 handler + 测试客户端。
type oidcTestEnv struct {
	router *gin.Engine
	svc    *Service
	cl     *Client
	secret string
}

func newOIDCTestEnv(t *testing.T, loggedIn bool) *oidcTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(sqliteOpen(t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Client{}, &AuthCode{}, &Token{}, &SigningKey{}))
	svc := NewService(db)
	require.NoError(t, svc.EnsureSigningKey(context.Background()))
	// 用户快照：模拟 sessions 表读取（token 用户信息来源）
	svc.SetUserSnapshot(func(_ context.Context, userID string) (*auth.CurrentUser, error) {
		if userID == "u-1" {
			return &auth.CurrentUser{ID: "u-1", Username: "alice", Email: "alice@corp.com", Roles: []string{"velora_admin"}}, nil
		}
		return &auth.CurrentUser{ID: userID}, nil
	})

	cl, secret, err := svc.CreateClient(context.Background(), 1,
		[]string{"https://app.example.com/callback", "http://localhost:9999/cb"},
		[]string{"authorization_code"})
	require.NoError(t, err)

	h := NewHandler(svc,
		func(c *gin.Context) *auth.CurrentUser {
			if loggedIn {
				return &auth.CurrentUser{ID: "u-1", Username: "alice", Email: "alice@corp.com", Roles: []string{"velora_admin"}}
			}
			return nil
		},
		func(_ *gin.Context, p string) string { return "/login?redirect=" + url.QueryEscape(p) },
		nil,
	)
	r := gin.New()
	h.Register(r.Group("/oidc"))
	return &oidcTestEnv{router: r, svc: svc, cl: cl, secret: secret}
}

const testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

func testChallenge() string {
	return s256Challenge(testVerifier)
}

// doAuthorize 发起 authorize 请求。
func (e *oidcTestEnv) doAuthorize(t *testing.T, params map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodGet, "/oidc/authorize?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

// authorizeBase 返回标准 authorize 参数（PKCE S256）。
func (e *oidcTestEnv) authorizeBase(redirect string) map[string]string {
	return map[string]string{
		"client_id":             e.cl.ClientID,
		"redirect_uri":          redirect,
		"response_type":         "code",
		"scope":                 "openid profile email",
		"state":                 "xyz-state",
		"code_challenge":        testChallenge(),
		"code_challenge_method": "S256",
	}
}

// TestAuthorizeRequiresLogin 未登录 → 302 到登录页（不泄露 code）。
func TestAuthorizeRequiresLogin(t *testing.T) {
	e := newOIDCTestEnv(t, false)
	w := e.doAuthorize(t, e.authorizeBase("https://app.example.com/callback"))
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login?redirect=")
	assert.NotContains(t, w.Body.String(), "code=")
}

// TestAuthorizeIssuesCode 已登录 → 302 携带一次性 code + state 回显。
func TestAuthorizeIssuesCode(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	w := e.doAuthorize(t, e.authorizeBase("https://app.example.com/callback"))
	assert.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	assert.Contains(t, loc, "code=")
	assert.Contains(t, loc, "state=xyz-state")
}

// TestAuthorizeRejectsBadRedirect 非白名单 redirect_uri → 400（不得跳转）。
func TestAuthorizeRejectsBadRedirect(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	bad := []string{
		"https://evil.example.com/cb",
		"https://app.example.com/callback.evil",
		"javascript:alert(1)",
		"",
		"http://localhost:9999/other", // 路径前缀不同也拒绝（精确匹配）
	}
	for _, u := range bad {
		w := e.doAuthorize(t, e.authorizeBase(u))
		assert.Equal(t, http.StatusBadRequest, w.Code, "redirect=%q 应拒绝", u)
		assert.NotContains(t, w.Header().Get("Location"), "code=")
	}
}

// TestAuthorizeRejectsNoPKCE 无 PKCE → 400。
func TestAuthorizeRejectsNoPKCE(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	p := e.authorizeBase("https://app.example.com/callback")
	delete(p, "code_challenge")
	delete(p, "code_challenge_method")
	w := e.doAuthorize(t, p)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "PKCE")
}

// TestTokenExchangeFullFlow code + verifier → token → userinfo（含 roles）。
func TestTokenExchangeFullFlow(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	redirect := "https://app.example.com/callback"
	authW := e.doAuthorize(t, e.authorizeBase(redirect))
	loc, _ := url.Parse(authW.Header().Get("Location"))
	code := loc.Query().Get("code")
	require.NotEmpty(t, code)

	// token 端点（client_secret_post）
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", e.cl.ClientID)
	form.Set("client_secret", e.secret)
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("code_verifier", testVerifier)
	req := httptest.NewRequest(http.MethodPost, "/oidc/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var tr struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &tr))
	assert.NotEmpty(t, tr.AccessToken)
	assert.NotEmpty(t, tr.RefreshToken)
	assert.Equal(t, "Bearer", tr.TokenType)

	// userinfo
	req2 := httptest.NewRequest(http.MethodGet, "/oidc/userinfo", nil)
	req2.Header.Set("Authorization", "Bearer "+tr.AccessToken)
	w2 := httptest.NewRecorder()
	e.router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	var info map[string]any
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &info))
	assert.Equal(t, "u-1", info["sub"])
	assert.Equal(t, "alice", info["preferred_username"])
	assert.Contains(t, info["roles"], "velora_admin")
}

// TestCodeSingleUse code 一次性：第二次兑换拒绝。
func TestCodeSingleUse(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	redirect := "https://app.example.com/callback"
	authW := e.doAuthorize(t, e.authorizeBase(redirect))
	loc, _ := url.Parse(authW.Header().Get("Location"))
	code := loc.Query().Get("code")

	exchange := func() int {
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("client_id", e.cl.ClientID)
		form.Set("client_secret", e.secret)
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("code_verifier", testVerifier)
		req := httptest.NewRequest(http.MethodPost, "/oidc/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		e.router.ServeHTTP(w, req)
		return w.Code
	}
	assert.Equal(t, http.StatusOK, exchange(), "第一次应成功")
	assert.Equal(t, http.StatusBadRequest, exchange(), "code 一次性：第二次应拒绝")
}

// TestTokenWrongSecret 错误 client_secret → 401。
func TestTokenWrongSecret(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	redirect := "https://app.example.com/callback"
	authW := e.doAuthorize(t, e.authorizeBase(redirect))
	loc, _ := url.Parse(authW.Header().Get("Location"))
	code := loc.Query().Get("code")

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", e.cl.ClientID)
	form.Set("client_secret", "wrong-secret")
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("code_verifier", testVerifier)
	req := httptest.NewRequest(http.MethodPost, "/oidc/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestTokenClientSecretBasic Basic 认证方式（client_secret_basic）。
func TestTokenClientSecretBasic(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	redirect := "https://app.example.com/callback"
	authW := e.doAuthorize(t, e.authorizeBase(redirect))
	loc, _ := url.Parse(authW.Header().Get("Location"))
	code := loc.Query().Get("code")

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirect)
	form.Set("code_verifier", testVerifier)
	req := httptest.NewRequest(http.MethodPost, "/oidc/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(e.cl.ClientID, e.secret)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUserinfoRequiresToken 无 token 或无效 token → 401。
func TestUserinfoRequiresToken(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	req := httptest.NewRequest(http.MethodGet, "/oidc/userinfo", nil)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/oidc/userinfo", nil)
	req2.Header.Set("Authorization", "Bearer invalid-token")
	w2 := httptest.NewRecorder()
	e.router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

// TestDiscovery jwks 发现文档 + 密钥端点。
func TestDiscoveryAndJWKS(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	req := httptest.NewRequest(http.MethodGet, "/oidc/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var doc struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.Contains(t, doc.Data["code_challenge_methods_supported"], "S256")
	assert.Contains(t, doc.Data["grant_types_supported"], "authorization_code")

	req2 := httptest.NewRequest(http.MethodGet, "/oidc/jwks", nil)
	w2 := httptest.NewRecorder()
	e.router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "\"keys\"")
}

// TestRefreshRotationRefresh 刷新轮换：旧 refresh 失效、新 refresh 可用。
func TestRefreshRotationRefresh(t *testing.T) {
	e := newOIDCTestEnv(t, true)
	redirect := "https://app.example.com/callback"
	authW := e.doAuthorize(t, e.authorizeBase(redirect))
	loc, _ := url.Parse(authW.Header().Get("Location"))
	code := loc.Query().Get("code")

	exchange := func() (int, string) {
		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("client_id", e.cl.ClientID)
		form.Set("client_secret", e.secret)
		form.Set("code", code)
		form.Set("redirect_uri", redirect)
		form.Set("code_verifier", testVerifier)
		req := httptest.NewRequest(http.MethodPost, "/oidc/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		e.router.ServeHTTP(w, req)
		var tr struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &tr)
		return w.Code, tr.RefreshToken
	}
	_, refresh1 := exchange()

	refresh := func(rt string) (int, string) {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("client_id", e.cl.ClientID)
		form.Set("client_secret", e.secret)
		form.Set("refresh_token", rt)
		req := httptest.NewRequest(http.MethodPost, "/oidc/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		e.router.ServeHTTP(w, req)
		var tr struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &tr)
		return w.Code, tr.RefreshToken
	}

	// 第一次 refresh：成功并轮换
	code1, refresh2 := refresh(refresh1)
	assert.Equal(t, http.StatusOK, code1)
	// 旧 refresh 已失效
	codeOld, _ := refresh(refresh1)
	assert.Equal(t, http.StatusBadRequest, codeOld, "旧 refresh_token 应失效")
	// 新 refresh 可用
	code2, _ := refresh(refresh2)
	assert.Equal(t, http.StatusOK, code2, "新 refresh_token 应可用")
}
