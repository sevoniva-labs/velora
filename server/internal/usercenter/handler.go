// Package usercenter 提供自助用户中心（Phase C4）。
//
// 能力：
//   - 修改密码（校验旧密码 → Casdoor 最小权限更新 → 吊销全部会话强制重登）
//   - 设备管理（会话列表/下线：复用 auth 会话 API）
//   - 个人资料展示（来自会话快照）
//
// 约束：所有写操作仅作用于当前登录用户；不暴露任何管理级端点。
package usercenter

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// PasswordUpdater 修改密码所需依赖（由组装层注入）。
type PasswordUpdater interface {
	UpdateUserPassword(ctx context.Context, userID, newPassword string) error
	VerifyPassword(ctx context.Context, username, password string) error
}

// Handler 提供自助用户中心端点。
type Handler struct {
	casdoor   PasswordUpdater
	session   *auth.SessionStore
	adminRole string
}

// NewHandler 创建用户中心 Handler。
func NewHandler(cas PasswordUpdater, session *auth.SessionStore, adminRole string) *Handler {
	return &Handler{casdoor: cas, session: session, adminRole: adminRole}
}

// Register 注册路由（受保护）。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/user-center")
	g.GET("/profile", h.profile)
	g.POST("/change-password", h.changePassword)
}

// profile 返回当前用户个人资料（会话快照 + 管理标记）。
func (h *Handler) profile(c *gin.Context) {
	u, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	type profileView struct {
		*auth.CurrentUser
		Admin bool `json:"admin"`
	}
	response.OK(c, &profileView{CurrentUser: u, Admin: u.IsAdmin(h.adminRole)})
}

// changePassword 修改当前用户密码。
// 流程：旧密码校验（ROPC）→ 新密码强度校验 → Casdoor 更新 → 吊销全部会话强制重登。
func (h *Handler) changePassword(c *gin.Context) {
	u, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	if body.OldPassword == "" || body.NewPassword == "" {
		response.Error(c, errs.InvalidParam("请输入旧密码和新密码"))
		return
	}
	if msg := validatePassword(body.NewPassword); msg != "" {
		response.Error(c, errs.New(errs.CodeInvalidParam, http.StatusBadRequest, msg))
		return
	}
	if body.OldPassword == body.NewPassword {
		response.Error(c, errs.InvalidParam("新密码不能与旧密码相同"))
		return
	}

	// 1) 旧密码校验（Casdoor ROPC）：防 CSRF 变更 / 非本人操作
	if err := h.casdoor.VerifyPassword(c.Request.Context(), u.Username, body.OldPassword); err != nil {
		response.Error(c, errs.New(errs.CodeLoginFailed, http.StatusForbidden, "当前密码不正确"))
		return
	}
	// 2) 最小权限更新（目标用户 == 当前登录用户，强校验）
	// Casdoor 用户 id 为 owner/name 格式；若 Organization 为空回退内置组织。
	owner := u.Organization
	if owner == "" {
		owner = "built-in"
	}
	if err := h.casdoor.UpdateUserPassword(c.Request.Context(), owner+"/"+u.Username, body.NewPassword); err != nil {
		slog.Error("Casdoor 改密失败", "user", u.Username, "err", err)
		response.Error(c, errs.Wrap(errs.CodeInternal, http.StatusBadGateway, "密码更新失败，请稍后再试", err))
		return
	}
	// 3) 吊销全部会话：改密后强制全端下线（含当前会话）
	if h.session != nil {
		_ = h.session.RevokeAllForUser(u.ID)
	}
	response.OK(c, gin.H{"status": "ok", "message": "密码已更新，请重新登录"})
}

// validatePassword 密码强度校验：8-72 位，含字母与数字。
func validatePassword(p string) string {
	if utf8.RuneCountInString(p) < 8 || len(p) > 72 {
		return "密码长度需为 8-72 个字符"
	}
	hasLetter := false
	hasDigit := false
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "密码需同时包含字母和数字"
	}
	_ = strings.TrimSpace(p)
	return ""
}
