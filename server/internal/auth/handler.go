package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// lockoutDefaultLockMinutes 锁定提示默认分钟数（RecordFailure 返回 locked 时无剩余时间信息）。
const lockoutDefaultLockMinutes = 15

// AuditCallback 为认证事件的审计回调（由组装层注入，避免 auth 依赖 audit 包）。
type AuditCallback func(c *gin.Context, userID string)

// Handler 提供认证相关 HTTP 端点。
type Handler struct {
	oidc             *OIDCManager
	sessions         *SessionStore
	adminRole        string
	defaultRedirect  string
	onLogin          AuditCallback
	onLogout         AuditCallback
	onLoginFailed    AuditCallback  // 登录失败审计（Phase C5：LOGIN_FAILED）
	lockout          LockoutManager // 账户锁定（nil 时不启用）
	turnstile        TurnstileVerifier
	turnstileSiteKey string // 公开 site key（前端 widget 渲染；未配置时不启用）
}

// LockoutManager 为账户锁定接口（由组装层注入，避免 auth 依赖 lockout 包）。
type LockoutManager interface {
	IsLocked(ctx context.Context, username string) (bool, time.Duration, error)
	RecordFailure(ctx context.Context, username string) (locked bool, err error)
	RecordSuccess(ctx context.Context, username string) error
}

// TurnstileVerifier 为 Cloudflare Turnstile 人机验证接口（由组装层注入）。
type TurnstileVerifier interface {
	Enabled() bool
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}

// NewHandler 创建认证 Handler。
func NewHandler(oidc *OIDCManager, sessions *SessionStore, adminRole, defaultRedirect string, onLogin, onLogout, onLoginFailed AuditCallback, lockout LockoutManager) *Handler {
	return &Handler{
		oidc:            oidc,
		sessions:        sessions,
		adminRole:       adminRole,
		defaultRedirect: defaultRedirect,
		onLogin:         onLogin,
		onLogout:        onLogout,
		onLoginFailed:   onLoginFailed,
		lockout:         lockout,
	}
}

// WithTurnstile 启用登录人机验证（配置了 site/secret key 后由组装层调用）。
func (h *Handler) WithTurnstile(v TurnstileVerifier, siteKey string) *Handler {
	h.turnstile = v
	h.turnstileSiteKey = siteKey
	return h
}

// Register 注册受保护路由（login/callback 为公开端点，由 httpserver 组装时注册）。
// Register 注册受保护路由（login/callback 为公开端点，由 httpserver 组装时注册）。
func (h *Handler) Register(r gin.IRouter) {
	r.POST("/auth/logout", h.logout)
	r.GET("/me", h.me)
	// 会话管理（Phase C1）：设备列表 / 强制下线
	r.GET("/auth/sessions", h.listSessions)
	r.DELETE("/auth/sessions/:sid", h.revokeSession)
	r.DELETE("/auth/sessions", h.revokeAllSessions)
}

// RegisterPublic 注册公开端点：登录页人机验证配置（仅暴露 site key，secret 永不下发）。
func (h *Handler) RegisterPublic(r gin.IRouter) {
	r.GET("/auth/turnstile-config", h.turnstileConfig)
}

// turnstileConfig 返回登录人机验证配置（未启用时 enabled=false，前端不渲染 widget）。
func (h *Handler) turnstileConfig(c *gin.Context) {
	// 配置可能动态变化：禁止中间层缓存（登录页每次实时读取）。
	c.Header("Cache-Control", "no-store")
	enabled := h.turnstile != nil && h.turnstile.Enabled() && h.turnstileSiteKey != ""
	response.OK(c, gin.H{"enabled": enabled, "siteKey": h.turnstileSiteKey})
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
	encoded, err := h.sessions.EncodeWithMeta(session, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.Error(c, errs.Internal("会话创建失败", err))
		return
	}
	path, maxAge, secure, domain := h.sessions.CookieOptions()
	// 显式 SameSite=Lax：顶层导航可携带（OIDC 回跳），并抑制跨站写请求携带。
	c.SetSameSite(http.SameSiteLaxMode)
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

// login 通过账号密码（Casdoor ROPC 代理）登录并建立会话。
// 公开端点（无需会话/CSRF），由 httpserver 组装时注册并附加限流。
func (h *Handler) LoginWithPassword(c *gin.Context) {
	var body struct {
		Username       string `json:"username"`
		Password       string `json:"password"`
		Redirect       string `json:"redirect"`
		TurnstileToken string `json:"turnstileToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		response.Error(c, errs.New(errs.CodeLoginFailed, http.StatusBadRequest, "请输入账号和密码"))
		return
	}

	// Cloudflare Turnstile 人机验证（Phase：登录安全加固）：配置启用后强制校验，
	// 防 bot 撞库/分布式暴力破解（IP 限流可被轮换 IP 绕过，验证码是可靠门禁）。
	// 拒绝路径与登录失败同等对待：入审计（LOGIN_FAILED）+ 失败计数（安全事件统一口径）。
	if h.turnstile != nil && h.turnstile.Enabled() {
		reject := func(msg string) {
			metrics.Emit("velora_auth_login_failure_total")
			if h.onLoginFailed != nil {
				h.onLoginFailed(c, body.Username)
			}
			response.ErrorWith(c, http.StatusForbidden, errs.CodeTurnstile, msg)
		}
		if strings.TrimSpace(body.TurnstileToken) == "" {
			// 缺 token 直接拒绝，避免无谓的 siteverify 调用。
			reject("请完成人机验证后重试")
			return
		}
		ok, verr := h.turnstile.Verify(c.Request.Context(), body.TurnstileToken, c.ClientIP())
		if verr != nil {
			// 验证服务故障：拒绝登录（fail-closed），宁可误伤不放行 bot。
			slog.Warn("人机验证服务异常，拒绝登录", "err", verr)
			reject("人机验证服务异常，请稍后重试")
			return
		}
		if !ok {
			reject("请完成人机验证后重试")
			return
		}
	}

	// 账户锁定检查（Phase C3）：锁定期间直接拒绝，不访问 Casdoor。
	if h.lockout != nil {
		locked, ttl, err := h.lockout.IsLocked(c.Request.Context(), body.Username)
		if err != nil {
			slog.Warn("账户锁定状态查询失败", "username", body.Username, "err", err)
		} else if locked {
			metrics.Emit("velora_auth_login_failure_total")
			if h.onLoginFailed != nil {
				h.onLoginFailed(c, body.Username)
			}
			minutes := int(ttl.Minutes()) + 1
			response.ErrorWith(c, http.StatusTooManyRequests, errs.CodeRateLimited,
				fmt.Sprintf("登录失败次数过多，账户已锁定，请 %d 分钟后再试", minutes))
			return
		}
	}

	user, err := h.oidc.LoginWithPassword(c.Request.Context(), body.Username, body.Password)
	if err != nil {
		metrics.Emit("velora_auth_login_failure_total")
		if h.onLoginFailed != nil {
			h.onLoginFailed(c, body.Username)
		}
		// 记录失败（凭据错误时计数；服务异常不计入账户锁定，避免误锁）
		if h.lockout != nil && errs.Is(err, errs.CodeLoginFailed) {
			if locked, lerr := h.lockout.RecordFailure(c.Request.Context(), body.Username); lerr == nil && locked {
				response.ErrorWith(c, http.StatusTooManyRequests, errs.CodeRateLimited,
					fmt.Sprintf("登录失败次数过多，账户已锁定，请 %d 分钟后再试", int(lockoutDefaultLockMinutes)))
				return
			}
		}
		if errs.Is(err, errs.CodeLoginFailed) {
			response.Error(c, err)
			return
		}
		response.Error(c, errs.Wrap(errs.CodeOIDCTokenFailed, http.StatusBadGateway, "登录服务暂不可用，请稍后再试", err))
		return
	}
	metrics.Emit("velora_auth_login_success_total")
	if h.lockout != nil {
		_ = h.lockout.RecordSuccess(c.Request.Context(), body.Username)
	}

	session := h.sessions.NewSession(user)
	encoded, err := h.sessions.EncodeWithMeta(session, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		response.Error(c, errs.Internal("会话创建失败", err))
		return
	}
	path, maxAge, secure, domain := h.sessions.CookieOptions()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, encoded, maxAge, path, domain, secure, true)

	if csrf, err := RandomToken(16); err == nil {
		c.SetCookie(CSRFCookieName, csrf, maxAge, path, domain, secure, false)
	}

	if h.onLogin != nil {
		h.onLogin(c, user.ID)
	}

	response.OK(c, gin.H{"redirect": h.sanitizeRedirect(body.Redirect)})
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

// logout 注销当前会话（同时吊销服务端会话记录）。
func (h *Handler) logout(c *gin.Context) {
	u := UserIDFrom(c)
	// 服务端吊销：解析当前 cookie 中的 SID 并吊销
	if token, err := c.Cookie(SessionCookieName); err == nil && token != "" {
		if session, err := h.sessions.Decode(token); err == nil && session.SID != "" {
			_ = h.sessions.Revoke(session.SID)
		}
	}
	if h.onLogout != nil {
		h.onLogout(c, u)
	}
	path, _, secure, domain := h.sessions.CookieOptions()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, path, domain, secure, true)
	c.SetCookie(CSRFCookieName, "", -1, path, domain, secure, false)
	response.OK(c, gin.H{"status": "ok"})
}

// me 返回当前登录用户（含 admin 标记，供前端控制管理入口展示）。
func (h *Handler) me(c *gin.Context) {
	u, err := RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	type meView struct {
		*CurrentUser
		Admin bool `json:"admin"`
	}
	response.OK(c, &meView{CurrentUser: u, Admin: u.IsAdmin(h.adminRole)})
}

// sanitizeRedirect 仅允许站内相对路径，防 Open Redirect。
//
// 严格校验（url.Parse + 字符黑名单），杜绝反斜杠 / 百分号编码绕过：
//   - 必须以 / 开头、无 scheme、无 host（含 \ 与 %2f/%5c 解码后的变体）
//   - 原始串不得包含 \、"//"、":"、%2f、%5c（大小写不敏感）
func (h *Handler) sanitizeRedirect(redirect string) string {
	redirect = strings.TrimSpace(redirect)
	if redirect == "" {
		return h.defaultRedirect
	}
	lower := strings.ToLower(redirect)
	if strings.ContainsAny(redirect, "\\:") ||
		strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return h.defaultRedirect
	}
	u, err := url.Parse(redirect)
	if err != nil || u.Scheme != "" || u.Host != "" || u.IsAbs() || !strings.HasPrefix(redirect, "/") {
		return h.defaultRedirect
	}
	return redirect
}

// listSessions 列出当前用户全部会话（设备列表）。
func (h *Handler) listSessions(c *gin.Context) {
	u, err := RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	list, err := h.sessions.ListForUser(u.ID)
	if err != nil {
		response.Error(c, errs.DB(err))
		return
	}
	type sessionView struct {
		SessionID    string     `json:"sessionId"`
		UserAgent    string     `json:"userAgent"`
		IP           string     `json:"ip"`
		LastActiveAt time.Time  `json:"lastActiveAt"`
		ExpiresAt    time.Time  `json:"expiresAt"`
		RevokedAt    *time.Time `json:"revokedAt,omitempty"`
		Current      bool       `json:"current"`
	}
	currentSID := ""
	if token, err := c.Cookie(SessionCookieName); err == nil && token != "" {
		if session, err := h.sessions.Decode(token); err == nil {
			currentSID = session.SID
		}
	}
	out := make([]sessionView, 0, len(list))
	for _, r := range list {
		out = append(out, sessionView{
			SessionID:    r.SessionID,
			UserAgent:    r.UserAgent,
			IP:           r.IP,
			LastActiveAt: r.LastActiveAt,
			ExpiresAt:    r.ExpiresAt,
			RevokedAt:    r.RevokedAt,
			Current:      r.SessionID == currentSID,
		})
	}
	response.OK(c, out)
}

// revokeSession 强制下线指定会话（仅本人会话，管理员可另行扩展）。
func (h *Handler) revokeSession(c *gin.Context) {
	u, err := RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	sid := strings.TrimSpace(c.Param("sid"))
	if sid == "" {
		response.Error(c, errs.InvalidParam("会话 ID 无效"))
		return
	}
	// 仅允许吊销本人会话
	list, err := h.sessions.ListForUser(u.ID)
	if err != nil {
		response.Error(c, errs.DB(err))
		return
	}
	owned := false
	for _, r := range list {
		if r.SessionID == sid {
			owned = true
			break
		}
	}
	if !owned {
		response.Error(c, errs.New(errs.CodeForbidden, http.StatusForbidden, "无权吊销该会话"))
		return
	}
	if err := h.sessions.Revoke(sid); err != nil {
		response.Error(c, errs.DB(err))
		return
	}
	if h.onLogout != nil {
		h.onLogout(c, u.ID)
	}
	response.OK(c, gin.H{"revoked": sid})
}

// revokeAllSessions 强制下线本人全部会话（改密后全端下线）。
func (h *Handler) revokeAllSessions(c *gin.Context) {
	u, err := RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	if err := h.sessions.RevokeAllForUser(u.ID); err != nil {
		response.Error(c, errs.DB(err))
		return
	}
	if h.onLogout != nil {
		h.onLogout(c, u.ID)
	}
	response.OK(c, gin.H{"status": "ok"})
}
