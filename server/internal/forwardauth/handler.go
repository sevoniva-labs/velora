// Package forwardauth 提供 Forward Auth 校验端点（Phase D1）。
//
// 面向非 OIDC 老系统：网关（Nginx/APISIX）配置 `auth_request /forward-auth`，
// Velora 校验会话 Cookie：
//   - 有效会话 → 200 + X-Velora-User / X-Velora-Email 身份头（供上游注入）；
//   - 无/失效会话 → 401（网关据配置转跳 Velora 登录页，登录后回跳 next）。
package forwardauth

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

// Handler 提供 Forward Auth 校验端点。
type Handler struct {
	sessions *auth.SessionStore
	// loginURL 未登录时的跳转地址模板（如 /login?redirect=%s）。
	loginURL string
}

// NewHandler 创建 Handler。
func NewHandler(sessions *auth.SessionStore, loginURL string) *Handler {
	return &Handler{sessions: sessions, loginURL: loginURL}
}

// Register 注册公开端点（无 CSRF；GET 只读校验）。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/forward-auth", h.check)
}

// check 校验会话：200（带身份头）或 401（带 Location 跳登录页）。
func (h *Handler) check(c *gin.Context) {
	token, err := c.Cookie(auth.SessionCookieName)
	if err != nil || token == "" {
		h.deny(c)
		return
	}
	session, err := h.sessions.Decode(token)
	if err != nil || session == nil {
		h.deny(c)
		return
	}
	user := session.ToCurrentUser()

	c.Header("X-Velora-User", user.Username)
	if user.Email != "" {
		c.Header("X-Velora-Email", user.Email)
	}
	if len(user.Roles) > 0 {
		for _, r := range user.Roles {
			c.Header("X-Velora-Role", r) // 多角色时合并逗号
		}
	}
	c.Status(http.StatusOK)
}

// deny 拒绝：401 + 登录跳转地址（网关据 Location 302）。
func (h *Handler) deny(c *gin.Context) {
	loc := h.loginURL
	if next := c.Query("next"); next != "" {
		if u, err := url.Parse(next); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			loc = h.loginURL + url.QueryEscape(next)
		}
	}
	c.Header("Location", loc)
	c.Status(http.StatusUnauthorized)
}
