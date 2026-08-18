package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// AuditCallback 为认证事件的审计回调（由组装层注入，避免 auth 依赖 audit 包）。
type AuditCallback func(c *gin.Context, userID string)

// Handler 提供认证相关 HTTP 端点。
type Handler struct {
	oidc            *OIDCManager
	sessions        *SessionStore
	adminRole       string
	defaultRedirect string
	onLogin         AuditCallback
	onLogout        AuditCallback
}

// NewHandler 创建认证 Handler。
func NewHandler(oidc *OIDCManager, sessions *SessionStore, adminRole, defaultRedirect string, onLogin, onLogout AuditCallback) *Handler {
	return &Handler{
		oidc:            oidc,
		sessions:        sessions,
		adminRole:       adminRole,
		defaultRedirect: defaultRedirect,
		onLogin:         onLogin,
		onLogout:        onLogout,
	}
}

// Register 注册路由。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/auth")
	g.GET("/oidc/login", h.Login)
	g.GET("/oidc/callback", h.Callback)
	g.POST("/logout", h.logout)
	r.GET("/me", h.me)
}

// login 发起 OIDC 登录跳转（Authorization Code + PKCE）。
func (h *Handler) Login(c *gin.Context) {
	redirect := h.sanitizeRedirect(c.Query("redirect"))
	loginURL, err := h.oidc.LoginURL(redirect)
	if err != nil {
		response.Error(c, errs.Wrap(errs.CodeOIDCInvalidParam, http.StatusBadRequest, "生成登录地址失败", err))
		return
	}
	c.Redirect(http.StatusFound, loginURL)
}

// callback 处理 Casdoor 回调：交换 token、建立会话、回跳。
func (h *Handler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		response.Error(c, errs.New(errs.CodeOIDCInvalidParam, http.StatusBadRequest, "回调缺少 code 或 state"))
		return
	}

	user, err := h.oidc.Exchange(c.Request.Context(), code, state)
	if err != nil {
		response.Error(c, errs.Wrap(errs.CodeOIDCTokenFailed, http.StatusBadRequest, "OIDC 登录失败", err))
		return
	}

	session := h.sessions.NewSession(user)
	encoded, err := h.sessions.Encode(session)
	if err != nil {
		response.Error(c, errs.Internal("会话创建失败", err))
		return
	}
	path, maxAge, secure, domain := h.sessions.CookieOptions()
	c.SetCookie(SessionCookieName, encoded, maxAge, path, domain, secure, true)

	// CSRF 双提交 Cookie。
	if csrf, err := RandomToken(16); err == nil {
		c.SetCookie(CSRFCookieName, csrf, maxAge, path, domain, secure, false)
	}

	if h.onLogin != nil {
		h.onLogin(c, user.ID)
	}

	c.Redirect(http.StatusFound, h.sanitizeRedirect(redirectFromState(state, h.oidc)))
}

// redirectFromState 从（已由 Exchange 验证的）state 中取回调落点。
// 为减少重复解析，这里从签名 state 解码；失败时回退默认路径。
func redirectFromState(stateToken string, m *OIDCManager) string {
	s, err := m.decodeState(stateToken)
	if err != nil {
		return ""
	}
	return s.Redirect
}

// logout 注销当前会话。
func (h *Handler) logout(c *gin.Context) {
	u := UserIDFrom(c)
	if h.onLogout != nil {
		h.onLogout(c, u)
	}
	path, _, secure, domain := h.sessions.CookieOptions()
	c.SetCookie(SessionCookieName, "", -1, path, domain, secure, true)
	c.SetCookie(CSRFCookieName, "", -1, path, domain, secure, false)
	response.OK(c, gin.H{"status": "ok"})
}

// me 返回当前登录用户。
func (h *Handler) me(c *gin.Context) {
	u, err := RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	response.OK(c, u)
}

// sanitizeRedirect 仅允许站内相对路径，防 Open Redirect。
func (h *Handler) sanitizeRedirect(redirect string) string {
	redirect = strings.TrimSpace(redirect)
	if redirect == "" {
		return h.defaultRedirect
	}
	// 必须为站内相对路径：以 / 开头，但不以 // 开头（防 protocol-relative），且不含 scheme。
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") && !strings.Contains(redirect, "://") {
		return redirect
	}
	return h.defaultRedirect
}
