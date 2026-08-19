package todo

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供待办 HTTP 端点。
type Handler struct {
	service   *Service
	audit     *audit.Service
	adminRole string
}

// NewHandler 创建待办 Handler。
func NewHandler(service *Service, auditSvc *audit.Service, adminRole string) *Handler {
	return &Handler{service: service, audit: auditSvc, adminRole: adminRole}
}

// Register 注册路由：用户查询/完成本人待办；写入端点限管理员（供外部系统对接集成）。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/todos")
	g.GET("", h.list)
	g.PATCH("/:id/done", h.markDone)
	g.POST("", permission.AdminRequired(h.adminRole), h.upsert)
}

// list 当前用户待办：?status=open|done|all&limit=N，附带待处理数量。
func (h *Handler) list(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	ctx := c.Request.Context()
	items, err := h.service.List(ctx, user.ID, c.DefaultQuery("status", StatusOpen), limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	openCount, err := h.service.OpenCount(ctx, user.ID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "openCount": openCount})
}

// upsertRequest 为外部系统推送待办的请求体。
type upsertRequest struct {
	UserID       string     `json:"userId"`
	Title        string     `json:"title"`
	SourceSystem string     `json:"sourceSystem"`
	SourceLabel  string     `json:"sourceLabel"`
	SourceID     string     `json:"sourceId"`
	Priority     string     `json:"priority"`
	Kind         string     `json:"kind"`
	URL          string     `json:"url"`
	DueAt        *time.Time `json:"dueAt"`
}

// upsert 创建/更新待办（管理员限定；幂等键 sourceSystem + sourceId + userId）。
func (h *Handler) upsert(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Title = strings.TrimSpace(req.Title)
	req.SourceSystem = strings.TrimSpace(req.SourceSystem)
	req.SourceID = strings.TrimSpace(req.SourceID)
	if req.UserID == "" || req.Title == "" || req.SourceSystem == "" || req.SourceID == "" {
		response.Error(c, errs.InvalidParam("userId、title、sourceSystem、sourceId 均为必填"))
		return
	}
	if len(req.Title) > 256 {
		response.Error(c, errs.InvalidParam("title 过长（最大 256 字符）"))
		return
	}
	priority := req.Priority
	if priority == "" {
		priority = PriorityMid
	}
	if !ValidPriority(priority) {
		response.Error(c, errs.InvalidParam("priority 仅支持 urgent/high/mid/low"))
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = KindOther
	}
	if !ValidKind(kind) {
		response.Error(c, errs.InvalidParam("kind 仅支持 mail/approval/devops/ops/project/hr/other"))
		return
	}
	todo, err := h.service.Upsert(c.Request.Context(), Todo{
		UserID:       req.UserID,
		Title:        req.Title,
		Kind:         kind,
		SourceSystem: req.SourceSystem,
		SourceLabel:  strings.TrimSpace(req.SourceLabel),
		SourceID:     req.SourceID,
		Priority:     priority,
		URL:          strings.TrimSpace(req.URL),
		DueAt:        req.DueAt,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionTodoUpsert,
		Resource:   "todo",
		ResourceID: strconv.FormatUint(todo.ID, 10),
	})
	metrics.Emit("velora_todo_upsert_total")
	response.OK(c, todo)
}

// markDone 将本人的待办标记为完成。
func (h *Handler) markDone(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errs.InvalidParam("无效的待办 ID"))
		return
	}
	if err := h.service.MarkDone(c.Request.Context(), user.ID, id); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionTodoDone,
		Resource:   "todo",
		ResourceID: strconv.FormatUint(id, 10),
	})
	metrics.Emit("velora_todo_done_total")
	response.OK(c, gin.H{"done": true})
}
