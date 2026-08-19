package privacy

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供数据导出端点（管理员）。
type Handler struct {
	svc       *Service
	audit     *audit.Service
	adminRole string
}

// NewHandler 创建导出 Handler。
func NewHandler(svc *Service, auditSvc *audit.Service, adminRole string) *Handler {
	return &Handler{svc: svc, audit: auditSvc, adminRole: adminRole}
}

// Register 注册路由。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/admin/users")
	g.Use(permission.AdminRequired(h.adminRole))
	g.GET("/:id/export", h.export)
}

// export 导出指定用户数据为 JSON 附件下载。
func (h *Handler) export(c *gin.Context) {
	userID := c.Param("id")
	data, err := h.svc.ExportUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	payload, err := data.MarshalJSON()
	if err != nil {
		response.Error(c, errs.Internal("导出数据序列化失败", err))
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   operatorOf(c),
		Action:     audit.ActionUserDataExport,
		Resource:   "user",
		ResourceID: userID,
		Detail:     "数据导出（GDPR/个保法）",
	})

	filename := fmt.Sprintf("velora-user-%s-%s.json", userID, time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
}

// operatorOf 读取当前管理员（由 AdminRequired 保证非空）。
func operatorOf(c *gin.Context) string {
	u := auth.CurrentUserFrom(c)
	if u == nil {
		return ""
	}
	return u.ID
}
