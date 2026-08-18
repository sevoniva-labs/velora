package mail

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供邮件 HTTP 端点。路由不暴露 Provider 细节（无 /api/aliyun/... 式端点）。
type Handler struct {
	service *Service
	audit   *audit.Service
}

// NewHandler 创建邮件 Handler。
func NewHandler(service *Service, auditSvc *audit.Service) *Handler {
	return &Handler{service: service, audit: auditSvc}
}

// Register 注册路由（均需登录；写操作走 CSRF 保护）。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/mail")
	g.GET("/providers", h.providers)
	g.GET("/accounts", h.listAccounts)
	g.POST("/accounts", h.createAccount)
	g.DELETE("/accounts/:id", h.deleteAccount)
	g.POST("/accounts/:id/test", h.testAccount)
	g.POST("/accounts/:id/sync", h.syncAccount)
	g.GET("/messages", h.listMessages)
	g.GET("/messages/:id", h.getMessage)
	g.POST("/messages/:id/read", h.setRead)
	g.POST("/messages/:id/star", h.setStar)
	g.POST("/messages/:id/todo", h.convertToTodo)
}

// providers 返回内置厂商 Profile + Provider 能力集（绑定表单数据源）。
func (h *Handler) providers(c *gin.Context) {
	response.OK(c, gin.H{
		"profiles":     BuiltinProfiles,
		"capabilities": NewIMAPProvider().Capabilities(),
	})
}

// listAccounts 我的邮箱账号。
func (h *Handler) listAccounts(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	accounts, err := h.service.ListAccounts(c.Request.Context(), user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, accounts)
}

// createAccountRequest 为绑定邮箱请求体。
type createAccountRequest struct {
	Provider    string `json:"provider"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	IMAPHost    string `json:"imapHost"`
	IMAPPort    int    `json:"imapPort"`
	SMTPHost    string `json:"smtpHost"`
	SMTPPort    int    `json:"smtpPort"`
}

// createAccount 绑定邮箱（先实测连接，通过后加密落库）。
func (h *Handler) createAccount(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	acc, err := h.service.CreateAccount(c.Request.Context(), user.ID, CreateAccountInput{
		Provider:    req.Provider,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		IMAPHost:    req.IMAPHost,
		IMAPPort:    req.IMAPPort,
		SMTPHost:    req.SMTPHost,
		SMTPPort:    req.SMTPPort,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionMailBind,
		Resource:   "mail_account",
		ResourceID: strconv.FormatUint(acc.ID, 10),
		Detail:     acc.Email,
	})
	response.OK(c, acc)
}

// deleteAccount 解绑邮箱。
func (h *Handler) deleteAccount(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteAccount(c.Request.Context(), user.ID, id); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionMailUnbind,
		Resource:   "mail_account",
		ResourceID: strconv.FormatUint(id, 10),
	})
	response.OK(c, gin.H{"deleted": true})
}

// testAccount 测试连接：始终 200，结果在 body 中（表单友好）。
func (h *Handler) testAccount(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.TestAccount(c.Request.Context(), user.ID, id); err != nil {
		response.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// syncAccount 手动触发同步。
func (h *Handler) syncAccount(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	acc, err := h.service.SyncAccount(c.Request.Context(), user.ID, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, acc)
}

// listMessages 邮件列表：?accountId=&unread=&starred=&keyword=&page=&pageSize=
func (h *Handler) listMessages(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	f := MessageFilter{
		AccountID: parseUintQuery(c, "accountId"),
		Keyword:   c.Query("keyword"),
		Page:      parseIntQuery(c, "page", 1),
		PageSize:  parseIntQuery(c, "pageSize", 20),
	}
	if v := c.Query("unread"); v != "" {
		b := v == "true" || v == "1"
		f.Unread = &b
	}
	if v := c.Query("starred"); v != "" {
		b := v == "true" || v == "1"
		f.Starred = &b
	}
	items, total, err := h.service.ListMessages(c.Request.Context(), user.ID, f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": f.Page, "pageSize": f.PageSize})
}

// getMessage 邮件详情（正文未缓存则按需拉取；打开即已读）。
func (h *Handler) getMessage(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	msg, bodyErr, err := h.service.GetMessage(c.Request.Context(), user.ID, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	resp := gin.H{"message": msg}
	if bodyErr != "" {
		resp["bodyError"] = bodyErr
	}
	response.OK(c, resp)
}

// setRead 设置已读/未读：body {read: bool}。
func (h *Handler) setRead(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Read *bool `json:"read"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Read == nil {
		response.Error(c, errs.InvalidParam("read 必填（true/false）"))
		return
	}
	if err := h.service.SetRead(c.Request.Context(), user.ID, id, *req.Read); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"read": *req.Read})
}

// setStar 设置星标：body {starred: bool}。
func (h *Handler) setStar(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Starred *bool `json:"starred"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Starred == nil {
		response.Error(c, errs.InvalidParam("starred 必填（true/false）"))
		return
	}
	if err := h.service.SetStarred(c.Request.Context(), user.ID, id, *req.Starred); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"starred": *req.Starred})
}

// convertRequest 为"转为待办"请求体。
type convertRequest struct {
	Title    string     `json:"title"`
	Priority string     `json:"priority"`
	Kind     string     `json:"kind"`
	DueAt    *time.Time `json:"dueAt"`
}

// convertToTodo 邮件转待办（幂等，重复转换不产生重复待办）。
func (h *Handler) convertToTodo(c *gin.Context) {
	user, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req convertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	t, err := h.service.ConvertToTodo(c.Request.Context(), user.ID, id, ConvertInput{
		Title:    req.Title,
		Priority: req.Priority,
		Kind:     req.Kind,
		DueAt:    req.DueAt,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionMailToTodo,
		Resource:   "mail_message",
		ResourceID: strconv.FormatUint(id, 10),
	})
	response.OK(c, t)
}

// ---------- 内部 ----------

func requireUser(c *gin.Context) (*auth.CurrentUser, bool) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return nil, false
	}
	return user, true
}

func parseID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errs.InvalidParam("无效的 ID"))
		return 0, false
	}
	return id, true
}

func parseUintQuery(c *gin.Context, key string) uint64 {
	n, _ := strconv.ParseUint(c.Query(key), 10, 64)
	return n
}

func parseIntQuery(c *gin.Context, key string, def int) int {
	v := strings.TrimSpace(c.Query(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}
