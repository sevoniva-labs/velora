package audit

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供审计日志 HTTP 端点（管理员）。
type Handler struct {
	service   *Service
	adminRole string
}

// NewHandler 创建审计 Handler。
func NewHandler(service *Service, adminRole string) *Handler {
	return &Handler{service: service, adminRole: adminRole}
}

// Register 注册路由。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/admin/audit-logs")
	g.Use(permission.AdminRequired(h.adminRole))
	g.GET("", h.list)
}

func (h *Handler) list(c *gin.Context) {
	filter := ListFilter{
		Operator: c.Query("operator"),
		Action:   c.Query("action"),
		Page:     parsePositive(c.Query("page"), 1),
		PageSize: parsePositive(c.Query("pageSize"), 20),
	}
	logs, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{
		"items":    logs,
		"total":    total,
		"page":     filter.Page,
		"pageSize": filter.PageSize,
	})
}

func (h *Handler) adminRequired() gin.HandlerFunc {
	return permission.AdminRequired(h.adminRole)
}

func parsePositive(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
