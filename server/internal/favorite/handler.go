package favorite

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/application"
	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Handler 提供收藏 HTTP 端点。
type Handler struct {
	service *Service
	appSvc  *application.Service
	appRepo *application.Repository
	audit   *audit.Service
}

// NewHandler 创建收藏 Handler。
func NewHandler(service *Service, appSvc *application.Service, appRepo *application.Repository, auditSvc *audit.Service) *Handler {
	return &Handler{service: service, appSvc: appSvc, appRepo: appRepo, audit: auditSvc}
}

// Register 注册路由。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/favorites")
	g.GET("", h.list)
	g.POST("/:applicationId", h.add)
	g.DELETE("/:applicationId", h.remove)
}

// list 我的收藏（复用应用列表查询的可见性过滤）。
func (h *Handler) list(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	page, err := h.appSvc.ListPublic(c.Request.Context(), user, application.ListFilter{
		FavoritesOnly: true,
		Page:          getIntParam(c, "page", 1),
		PageSize:      getIntParam(c, "pageSize", 100),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, page)
}

// add 收藏应用。
func (h *Handler) add(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	appID, ok := parseAppID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	app, err := h.appRepo.Get(ctx, appID)
	if err != nil {
		response.Error(c, err)
		return
	}
	if !application.CanAccess(user, app.Policies) {
		response.Error(c, errs.Forbidden("无权访问该应用"))
		return
	}
	if app.Status != application.StatusEnabled {
		response.Error(c, errs.New(errs.CodeApplicationNotFound, http.StatusNotFound, "应用不可用"))
		return
	}
	if err := h.service.Add(ctx, user.ID, appID); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionFavoriteAdd,
		Resource:   "application",
		ResourceID: strconv.FormatUint(appID, 10),
	})
	response.OK(c, gin.H{"favorited": true})
}

// remove 取消收藏。
func (h *Handler) remove(c *gin.Context) {
	user, err := auth.RequireUser(c)
	if err != nil {
		response.Error(c, errs.Unauthorized(""))
		return
	}
	appID, ok := parseAppID(c)
	if !ok {
		return
	}
	if err := h.service.Remove(c.Request.Context(), user.ID, appID); err != nil {
		response.Error(c, err)
		return
	}
	h.audit.Record(c, audit.Entry{
		Operator:   user.ID,
		Action:     audit.ActionFavoriteRemove,
		Resource:   "application",
		ResourceID: strconv.FormatUint(appID, 10),
	})
	response.OK(c, gin.H{"favorited": false})
}

func parseAppID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("applicationId"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, errs.InvalidParam("无效的应用 ID"))
		return 0, false
	}
	return id, true
}

// getIntParam 解析正整数查询参数，非法或缺失时返回默认值。
func getIntParam(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}
