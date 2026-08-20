package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTurnstile 可控的 TurnstileVerifier（测试替身）。
type mockTurnstile struct {
	enabled     bool
	verifyOK    bool
	verifyErr   error
	verifyCalls int
}

func (m *mockTurnstile) Enabled() bool { return m.enabled }
func (m *mockTurnstile) Verify(_ context.Context, token string, _ string) (bool, error) {
	m.verifyCalls++
	if token == "" {
		return false, nil // 与真实 Verifier 一致：空 token 不通过
	}
	return m.verifyOK, m.verifyErr
}

// newROPCProvider 模拟 Casdoor 的 password grant 端点（ROPC 登录）。
func newROPCProvider(t *testing.T) *httptest.Server {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                srv.URL,
			"authorization_endpoint":                srv.URL + "/authorize",
			"token_endpoint":                        srv.URL + "/token",
			"userinfo_endpoint":                     srv.URL + "/userinfo",
			"jwks_uri":                              srv.URL + "/jwks",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"keys": []any{rsaPublicJWK(&priv.PublicKey)}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "password" {
			http.Error(w, "bad grant", 400)
			return
		}
		now := time.Now().Unix()
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT","kid":"mock-kid"}`))
		claims, _ := json.Marshal(map[string]any{
			"iss":                srv.URL,
			"sub":                "casdoor-user-42",
			"aud":                "test-client",
			"exp":                now + 3600,
			"iat":                now,
			"preferred_username": "carson",
			"email":              "carson@example.com",
			"roles":              []string{"velora_admin"},
		})
		content := header + "." + base64.RawURLEncoding.EncodeToString(claims)
		sig := signRS256(t, priv, content)
		writeJSON(w, map[string]any{
			"access_token": "mock-access",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     content + "." + sig,
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newTurnstileTestHandler 组装带 Turnstile 的认证 Handler（ROPC mock + 内存会话）。
func newTurnstileTestHandler(t *testing.T, ts *mockTurnstile, siteKey string) (*gin.Engine, *mockTurnstile) {
	return newTurnstileTestHandlerAudit(t, ts, siteKey, nil)
}

// newTurnstileTestHandlerAudit 额外允许注入 onLoginFailed 计数（审计断言用）。
func newTurnstileTestHandlerAudit(t *testing.T, ts *mockTurnstile, siteKey string, auditCount *int) (*gin.Engine, *mockTurnstile) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	provider := newROPCProvider(t)

	oidc := NewOIDCManager(provider.URL, "test-client", "test-secret", "http://localhost/cb", time.Minute)
	sessions, err := NewSessionStore("0123456789abcdef0123456789abcdef", time.Hour, false, "")
	require.NoError(t, err)

	onFailed := func(c *gin.Context, username string) {
		if auditCount != nil {
			*auditCount++
		}
	}
	h := NewHandler(oidc, sessions, "velora_admin", "/", nil, onFailed, onFailed, nil)
	if ts != nil && ts.enabled {
		h.WithTurnstile(ts, siteKey)
	}

	r := gin.New()
	h.RegisterPublic(r.Group("/api/v1"))
	api := r.Group("/api/v1")
	api.POST("/auth/login", h.LoginWithPassword)
	return r, ts
}

func doLogin(t *testing.T, r *gin.Engine, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"username":"carson","password":"secret123"}`
	if token != "" {
		body = `{"username":"carson","password":"secret123","turnstileToken":"` + token + `"}`
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLoginWithoutTurnstile(t *testing.T) {
	// 未配置：登录正常（无验证码要求）。
	r, _ := newTurnstileTestHandler(t, nil, "")
	w := doLogin(t, r, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "000000")
}

func TestLoginTurnstileMissingToken(t *testing.T) {
	ts := &mockTurnstile{enabled: true, verifyOK: true}
	audited := 0
	r, _ := newTurnstileTestHandlerAudit(t, ts, "site-key-1", &audited)
	w := doLogin(t, r, "")
	assert.Equal(t, http.StatusForbidden, w.Code, "启用验证后缺 token 应拒绝")
	assert.Contains(t, w.Body.String(), "A05007")
	assert.Zero(t, ts.verifyCalls, "缺 token 不应触发服务端校验调用")
	assert.Equal(t, 1, audited, "人机验证拒绝应入登录失败审计")
}

func TestLoginTurnstileInvalidToken(t *testing.T) {
	ts := &mockTurnstile{enabled: true, verifyOK: false}
	r, _ := newTurnstileTestHandler(t, ts, "site-key-1")
	w := doLogin(t, r, "bad-token")
	assert.Equal(t, http.StatusForbidden, w.Code, "验证未通过应拒绝")
	assert.Contains(t, w.Body.String(), "A05007")
	assert.Equal(t, 1, ts.verifyCalls)
}

func TestLoginTurnstileServiceFailure(t *testing.T) {
	ts := &mockTurnstile{enabled: true, verifyOK: false, verifyErr: context.DeadlineExceeded}
	r, _ := newTurnstileTestHandler(t, ts, "site-key-1")
	w := doLogin(t, r, "token")
	assert.Equal(t, http.StatusForbidden, w.Code, "验证服务故障应 fail-closed 拒绝")
	assert.Contains(t, w.Body.String(), "A05007")
}

func TestLoginTurnstileValidToken(t *testing.T) {
	ts := &mockTurnstile{enabled: true, verifyOK: true}
	r, _ := newTurnstileTestHandler(t, ts, "site-key-1")
	w := doLogin(t, r, "valid-token")
	assert.Equal(t, http.StatusOK, w.Code, "验证通过后应放行登录")
	assert.Contains(t, w.Body.String(), "000000")
	assert.Equal(t, 1, ts.verifyCalls)
}

func TestTurnstileConfigEndpoint(t *testing.T) {
	// 未启用：enabled=false 且不下发 secret。
	r, _ := newTurnstileTestHandler(t, nil, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/turnstile-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
	assert.NotContains(t, w.Body.String(), "secret")

	// 启用：下发 site key。
	ts := &mockTurnstile{enabled: true}
	r2, _ := newTurnstileTestHandler(t, ts, "site-key-abc")
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/auth/turnstile-config", nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"enabled":true`)
	assert.Contains(t, w2.Body.String(), "site-key-abc")
	assert.NotContains(t, w2.Body.String(), "secret")
}

// signRS256 用私钥对 JWT signing input 做 RS256 签名（返回 base64url 签名段）。
func signRS256(t *testing.T, priv *rsa.PrivateKey, signingInput string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(sig)
}
