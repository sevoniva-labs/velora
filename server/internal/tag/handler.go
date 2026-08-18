package tag

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供标签 HTTP 端点。
type Handler struct {
	service   *Service
	audit     *audit.Service
	adminRole string
}

// NewHandler 创建标签 Handler。
func NewHandler(service *Service, auditSvc *audit.Service, adminRole string) *Handler {
	return &Handler{service: service, audit: auditSvc, adminRole: adminRole}
}

// Register 注册路由。GET 公开；写操作需管理员。
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/tags", h.list)

	admin := r.Group("/admin/tags")
	admin.Use(h.adminRequired())
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	tags, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tags)
}

func (h *Handler) create(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	t, err := h.service.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionTagCreate,
		Resource:   "tag",
		ResourceID: strconv.FormatUint(t.ID, 10),
		Detail:     t.Code,
	})
	response.Created(c, t)
}

func (h *Handler) update(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	t, err := h.service.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionTagUpdate,
		Resource:   "tag",
		ResourceID: strconv.FormatUint(id, 10),
		Detail:     t.Code,
	})
	response.OK(c, t)
}

func (h *Handler) delete(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionTagDelete,
		Resource:   "tag",
		ResourceID: strconv.FormatUint(id, 10),
	})
	response.NoContent(c)
}

func (h *Handler) adminRequired() gin.HandlerFunc {
	return permission.AdminRequired(h.adminRole)
}

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errs.InvalidParam("无效的资源 ID"))
		return 0, false
	}
	return id, true
}
