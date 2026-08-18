package application

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// VisitRecorder 由 visit 包实现，避免 handler 直接依赖 visit 包。
type VisitRecorder interface {
	RecordLaunch(ctx context.Context, userID string, appID uint64) error
}

// Handler 为应用 HTTP 端点。
type Handler struct {
	service   *Service
	repo      *Repository
	visits    VisitRecorder
	audit     *audit.Service
	adminRole string
}

// NewHandler 创建应用 Handler。
func NewHandler(service *Service, repo *Repository, visits VisitRecorder, auditSvc *audit.Service, adminRole string) *Handler {
	return &Handler{service: service, repo: repo, visits: visits, audit: auditSvc, adminRole: adminRole}
}

// Register 注册路由。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/applications")
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.POST("/:id/launch", h.launch)

	r.GET("/recent", h.recent)
	r.GET("/popular", h.popular)

	admin := r.Group("/admin/applications")
	admin.Use(h.adminRequired())
	admin.GET("", h.adminList)
	admin.POST("", h.create)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
	admin.PUT("/:id/policies", h.setPolicies)
}

// recent 最近使用。
func (h *Handler) recent(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	limit := parsePositive(c.Query("limit"), 8)
	items, err := h.service.RecentForUser(c.Request.Context(), user, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// popular 热门应用（按访问量）。
func (h *Handler) popular(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	limit := parsePositive(c.Query("limit"), 8)
	items, err := h.service.PopularForUser(c.Request.Context(), user, limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// list 用户侧应用列表。
func (h *Handler) list(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	filter := ListFilter{
		Keyword:       c.Query("keyword"),
		CategoryID:    parseUint(c.Query("categoryId")),
		TagIDs:        parseUintList(c.QueryArray("tagId")),
		FeaturedOnly:  c.Query("featured") == "true",
		FavoritesOnly: c.Query("favorites") == "true",
		Page:          parsePositive(c.Query("page"), 1),
		PageSize:      parsePositive(c.Query("pageSize"), 24),
	}
	page, err := h.service.ListPublic(c.Request.Context(), user, filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, page)
}

// get 应用详情。
func (h *Handler) get(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	dto, err := h.service.GetPublic(c.Request.Context(), user, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dto)
}

// launch 应用启动：权限校验 → 状态校验 → 生成可信 URL → 记录访问与审计。
// 禁止接受客户端 URL 参数（防 Open Redirect）。
func (h *Handler) launch(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	app, err := h.repo.Get(ctx, id)
	if err != nil {
		response.Error(c, err)
		return
	}
	if app.Status != StatusEnabled {
		response.Error(c, errs.New(errs.CodeApplicationDisabled, 400, "该应用当前不可用"))
		return
	}
	if !CanAccess(user, app.Policies) {
		response.Error(c, errs.Forbidden("无权访问该应用"))
		return
	}

	result, err := h.service.launch.Launch(ctx, app, user)
	if err != nil {
		response.Error(c, err)
		return
	}

	if h.visits != nil {
		if err := h.visits.RecordLaunch(ctx, user.ID, app.ID); err != nil {
			// 访问记录失败不阻断启动。
			_ = err
		}
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionAppLaunch,
		Resource:   "application",
		ResourceID: strconv.FormatUint(app.ID, 10),
		Detail:     app.Code,
	})
	response.OK(c, result)
}

// adminList 管理员应用列表。
func (h *Handler) adminList(c *gin.Context) {
	filter := ListFilter{
		Keyword:    c.Query("keyword"),
		CategoryID: parseUint(c.Query("categoryId")),
		Page:       parsePositive(c.Query("page"), 1),
		PageSize:   parsePositive(c.Query("pageSize"), 20),
	}
	page, err := h.service.AdminList(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, page)
}

// create 创建应用。
func (h *Handler) create(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误: "+err.Error()))
		return
	}
	dto, err := h.service.Create(c.Request.Context(), user.ID, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionAppCreate,
		Resource:   "application",
		ResourceID: strconv.FormatUint(dto.ID, 10),
		Detail:     dto.Code,
	})
	response.Created(c, dto)
}

// update 更新应用。
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
		response.Error(c, errs.InvalidParam("请求体格式错误: "+err.Error()))
		return
	}
	dto, err := h.service.Update(c.Request.Context(), user.ID, id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionAppUpdate,
		Resource:   "application",
		ResourceID: strconv.FormatUint(id, 10),
		Detail:     dto.Code,
	})
	response.OK(c, dto)
}

// delete 删除应用。
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
		Action:     audit.ActionAppDelete,
		Resource:   "application",
		ResourceID: strconv.FormatUint(id, 10),
	})
	response.NoContent(c)
}

// setPolicies 更新访问策略。
func (h *Handler) setPolicies(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		Policies []PolicyDTO `json:"policies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误: "+err.Error()))
		return
	}
	if err := h.service.SetPolicies(c.Request.Context(), id, req.Policies); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionPermissionChange,
		Resource:   "application",
		ResourceID: strconv.FormatUint(id, 10),
		Detail:     "policies updated",
	})
	response.OK(c, gin.H{"status": "ok"})
}

// adminRequired 管理员中间件。
func (h *Handler) adminRequired() gin.HandlerFunc {
	return permission.AdminRequired(h.adminRole)
}

// --- 参数解析工具 ---

func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errs.InvalidParam("无效的资源 ID"))
		return 0, false
	}
	return id, true
}

func parseUint(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
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

func parseUintList(values []string) []uint64 {
	var out []uint64
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64); err == nil && n > 0 {
				out = append(out, n)
			}
		}
	}
	return out
}

var _ = http.StatusOK
