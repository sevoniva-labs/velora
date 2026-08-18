package portal

import (
	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供门户设置与概览 HTTP 端点。
type Handler struct {
	service   *Service
	audit     *audit.Service
	adminRole string
}

// NewHandler 创建门户 Handler。
func NewHandler(service *Service, auditSvc *audit.Service, adminRole string) *Handler {
	return &Handler{service: service, audit: auditSvc, adminRole: adminRole}
}

// RegisterPublic 注册公开路由：门户展示配置（名称/公告等），登录页等无需登录即可读取。
func (h *Handler) RegisterPublic(r gin.IRouter) {
	r.GET("/portal/settings", h.all)
}

// Register 注册受保护路由。
func (h *Handler) Register(r gin.IRouter) {
	admin := r.Group("/admin")
	admin.Use(h.adminRequired())
	admin.GET("/dashboard", h.dashboard)
	admin.PUT("/portal/settings", h.updateSettings)
}

// all 返回门户公开设置（名称/公告/缩放等展示配置，公开只读）。
func (h *Handler) all(c *gin.Context) {
	list, err := h.service.All(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// dashboard 管理员概览。
func (h *Handler) dashboard(c *gin.Context) {
	stats, err := h.service.Dashboard(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, stats)
}

// updateSettings 更新门户设置。
func (h *Handler) updateSettings(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	if err := h.service.Set(c.Request.Context(), req.Key, req.Value); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionSettingUpdate,
		Resource:   "portal_settings",
		ResourceID: req.Key,
	})
	response.OK(c, gin.H{"key": req.Key, "value": req.Value})
}

func (h *Handler) adminRequired() gin.HandlerFunc {
	return permission.AdminRequired(h.adminRole)
}
