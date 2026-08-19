package oidcprovider

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 为 OIDC Provider 的 HTTP 路由。
type Handler struct {
	svc *Service
	// authorizeUser 回调：从 Velora 会话解析当前用户；返回 nil 表示未登录。
	// 由组装层注入（httpserver 通过 auth 中间件上下文实现），避免包循环。
	authorizeUser func(c *gin.Context) *auth.CurrentUser
	// loginRedirect 回调：生成"去登录"跳转 URL（带原 authorize 参数）。
	loginRedirect func(c *gin.Context, authorizePath string) string
	// audit 审计回调（可选）。
	audit func(c *gin.Context, action, resource, detail string)
}

// NewHandler 创建 OIDC Provider 路由处理器。
func NewHandler(svc *Service, authorizeUser func(*gin.Context) *auth.CurrentUser, loginRedirect func(*gin.Context, string) string, audit func(*gin.Context, string, string, string)) *Handler {
	if authorizeUser == nil {
		authorizeUser = func(*gin.Context) *auth.CurrentUser { return nil }
	}
	if loginRedirect == nil {
		loginRedirect = func(_ *gin.Context, p string) string { return "/login?redirect=" + url.QueryEscape(p) }
	}
	if audit == nil {
		audit = func(*gin.Context, string, string, string) {}
	}
	return &Handler{svc: svc, authorizeUser: authorizeUser, loginRedirect: loginRedirect, audit: audit}
}

// Register 注册 OIDC Provider 路由（挂到 /oidc 前缀，公开无需登录）。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/.well-known/openid-configuration", h.discovery)
	r.GET("/authorize", h.authorize)
	r.POST("/token", h.token)
	r.GET("/userinfo", h.userinfo)
	r.GET("/jwks", h.jwks)
	r.POST("/logout", h.logout)
}

// discovery OIDC 发现文档。
func (h *Handler) discovery(c *gin.Context) {
	iss := h.svc.issuer()
	doc := map[string]any{
		"issuer":                                iss,
		"authorization_endpoint":                iss + "/oidc/authorize",
		"token_endpoint":                        iss + "/oidc/token",
		"userinfo_endpoint":                     iss + "/oidc/userinfo",
		"jwks_uri":                              iss + "/oidc/jwks",
		"end_session_endpoint":                  iss + "/oidc/logout",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
		"claims_supported":                      []string{"sub", "preferred_username", "email", "roles", "groups"},
	}
	response.OK(c, doc)
}

// authorize 授权端点：校验客户端参数 → 已登录签发 code / 未登录跳登录。
func (h *Handler) authorize(c *gin.Context) {
	metricsEmit("velora_oidc_authorize_total")

	clientID := strings.TrimSpace(c.Query("client_id"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	responseType := strings.TrimSpace(c.Query("response_type"))
	scope := strings.TrimSpace(c.Query("scope"))
	state := strings.TrimSpace(c.Query("state"))
	nonce := strings.TrimSpace(c.Query("nonce"))
	challenge := strings.TrimSpace(c.Query("code_challenge"))
	challengeMethod := strings.TrimSpace(c.Query("code_challenge_method"))

	// 参数校验失败 → 直接错误页（无法安全重定向时不得携带用户跳转）。
	if responseType != "code" {
		h.audit(c, "OIDC_AUTHORIZE", "oidc", "response_type 不支持")
		metricsEmit("velora_oidc_authorize_failure_total")
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_response_type", "error_description": "仅支持 code"})
		return
	}
	if challenge == "" || challengeMethod != "S256" {
		h.audit(c, "OIDC_AUTHORIZE", "oidc", "缺少 PKCE")
		metricsEmit("velora_oidc_authorize_failure_total")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "必须使用 PKCE (S256)"})
		return
	}
	cl, err := h.svc.GetClientByID(c.Request.Context(), clientID)
	if err != nil {
		h.audit(c, "OIDC_AUTHORIZE", "oidc", "client 无效: "+clientID)
		metricsEmit("velora_oidc_authorize_failure_total")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client", "error_description": "客户端不存在或已禁用"})
		return
	}
	if !redirectAllowed(cl, redirectURI) {
		h.audit(c, "OIDC_AUTHORIZE", "oidc", "redirect_uri 不在白名单: "+redirectURI)
		metricsEmit("velora_oidc_authorize_failure_total")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "redirect_uri 不在白名单"})
		return
	}

	user := h.authorizeUser(c)
	if user == nil {
		// 未登录：跳 Velora 登录页，登录后回到 authorize
		authorizePath := "/oidc/authorize?" + c.Request.URL.RawQuery
		c.Redirect(http.StatusFound, h.loginRedirect(c, authorizePath))
		return
	}

	// 已登录：签发授权码
	code, err := h.svc.IssueCode(c.Request.Context(), cl, user, redirectURI, scope, challenge, challengeMethod, nonce)
	if err != nil {
		metricsEmit("velora_oidc_authorize_failure_total")
		h.audit(c, "OIDC_AUTHORIZE", "oidc", "code 签发失败: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	h.audit(c, "OIDC_AUTHORIZE", "oidc", "user="+user.ID+" client="+clientID)
	u := url.Values{}
	u.Set("code", code)
	if state != "" {
		u.Set("state", state)
	}
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	c.Redirect(http.StatusFound, redirectURI+sep+u.Encode())
}

// token 令牌端点：code 换 token / refresh 换 token。
func (h *Handler) token(c *gin.Context) {
	c.Request.ParseForm()
	grantType := strings.TrimSpace(c.PostForm("grant_type"))
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	clientSecret := strings.TrimSpace(c.PostForm("client_secret"))

	// 兼容 client_secret_basic：从 Authorization 头提取
	if clientID == "" {
		if id, secret, ok := basicAuth(c.GetHeader("Authorization")); ok {
			clientID, clientSecret = id, secret
		}
	}

	start := time.Now()
	defer func() {
		metricsObserve("velora_oidc_token_duration_milliseconds", float64(time.Since(start).Milliseconds()))
	}()

	cl, err := h.svc.authenticateClient(c.Request.Context(), clientID, clientSecret)
	if err != nil {
		metricsEmit("velora_oidc_token_failure_total")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
		return
	}

	switch grantType {
	case GrantTypeAuthorizationCode:
		code := strings.TrimSpace(c.PostForm("code"))
		redirectURI := strings.TrimSpace(c.PostForm("redirect_uri"))
		verifier := strings.TrimSpace(c.PostForm("code_verifier"))
		if code == "" || verifier == "" {
			metricsEmit("velora_oidc_token_failure_total")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "缺少 code 或 code_verifier"})
			return
		}
		_, access, refresh, err := h.svc.ExchangeCode(c.Request.Context(), cl, code, redirectURI, verifier)
		if err != nil {
			metricsEmit("velora_oidc_token_failure_total")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": err.Error()})
			return
		}
		metricsEmit("velora_oidc_token_total")
		h.audit(c, "OIDC_TOKEN", "oidc", "grant=code client="+clientID)
		c.JSON(http.StatusOK, tokenResponse(access, refresh, "Bearer", int(accessTokenTTL.Seconds())))

	case GrantTypeRefreshToken:
		refresh := strings.TrimSpace(c.PostForm("refresh_token"))
		if refresh == "" {
			metricsEmit("velora_oidc_token_failure_total")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "error_description": "缺少 refresh_token"})
			return
		}
		_, access, newRefresh, err := h.svc.RefreshToken(c.Request.Context(), cl, refresh)
		if err != nil {
			metricsEmit("velora_oidc_token_failure_total")
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant", "error_description": err.Error()})
			return
		}
		metricsEmit("velora_oidc_token_total")
		h.audit(c, "OIDC_TOKEN", "oidc", "grant=refresh client="+clientID)
		c.JSON(http.StatusOK, tokenResponse(access, newRefresh, "Bearer", int(accessTokenTTL.Seconds())))

	default:
		metricsEmit("velora_oidc_token_failure_total")
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
	}
}

// userinfo 用户信息端点（Bearer access_token）。
func (h *Handler) userinfo(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token"})
		return
	}
	claims, err := h.svc.VerifyAccessToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_token", "error_description": err.Error()})
		return
	}
	info := map[string]any{
		"sub":                claims.Subject,
		"preferred_username": claims.Username,
		"email":              claims.Email,
		"roles":              claims.Roles,
		"groups":             claims.Groups,
	}
	if claims.Email == "" {
		delete(info, "email")
	}
	c.JSON(http.StatusOK, info)
}

// jwks 公钥列表。
func (h *Handler) jwks(c *gin.Context) {
	jwks, err := h.svc.JWKS(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
		return
	}
	c.JSON(http.StatusOK, jwks)
}

// logout RP-initiated logout：吊销用户令牌（简单实现：清 Velora 会话由前端处理）。
func (h *Handler) logout(c *gin.Context) {
	user := h.authorizeUser(c)
	if user != nil {
		_ = h.svc.RevokeUserTokens(c.Request.Context(), user.ID)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- 内部工具 ---

func tokenResponse(access, refresh, tokenType string, expiresIn int) map[string]any {
	out := map[string]any{
		"access_token": access,
		"token_type":   tokenType,
		"expires_in":   expiresIn,
	}
	if refresh != "" {
		out["refresh_token"] = refresh
	}
	return out
}

// redirectAllowed 校验 redirect_uri 严格匹配白名单（完整 URL 精确匹配或 path 前缀）。
func redirectAllowed(cl *Client, redirectURI string) bool {
	if redirectURI == "" {
		return false
	}
	// 防 Open Redirect：拒绝非 http(s)、拒绝 \\ 或 // 混淆
	u, err := url.Parse(redirectURI)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	if strings.Contains(redirectURI, "\\") || strings.Contains(redirectURI, "//") && !strings.HasPrefix(redirectURI, "http://") && !strings.HasPrefix(redirectURI, "https://") {
		return false
	}
	for _, allowed := range cl.RedirectURIs() {
		if allowed == redirectURI {
			return true
		}
	}
	return false
}

// basicAuth 解析 Basic 认证头。
func basicAuth(header string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
	if err != nil {
		return "", "", false
	}
	idx := strings.IndexByte(string(raw), ':')
	if idx < 0 {
		return "", "", false
	}
	return string(raw[:idx]), string(raw[idx+1:]), true
}

// bearerToken 提取 Bearer token。
func bearerToken(header string) string {
	const prefix = "Bearer "
	if strings.HasPrefix(header, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(header, prefix))
	}
	return ""
}

// metricsEmit 指标快捷封装。
func metricsEmit(name string) { metrics.Emit(name) }

// metricsObserve 指标观察快捷封装。
func metricsObserve(name string, v float64) { metrics.Observe(name, v) }
