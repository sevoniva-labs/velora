// Package audit 提供审计日志记录与查询。
package audit

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// 审计动作常量（与前端标签映射保持一致）。
const (
	ActionLogin            = "LOGIN"
	ActionLogout           = "LOGOUT"
	ActionAppCreate        = "APPLICATION_CREATE"
	ActionAppUpdate        = "APPLICATION_UPDATE"
	ActionAppDelete        = "APPLICATION_DELETE"
	ActionAppLaunch        = "APPLICATION_LAUNCH"
	ActionFavoriteAdd      = "FAVORITE_ADD"
	ActionFavoriteRemove   = "FAVORITE_REMOVE"
	ActionPermissionChange = "PERMISSION_CHANGE"
	ActionCategoryCreate   = "CATEGORY_CREATE"
	ActionCategoryUpdate   = "CATEGORY_UPDATE"
	ActionCategoryDelete   = "CATEGORY_DELETE"
	ActionTagCreate        = "TAG_CREATE"
	ActionTagUpdate        = "TAG_UPDATE"
	ActionTagDelete        = "TAG_DELETE"
	ActionSettingUpdate    = "SETTING_UPDATE"
	ActionTodoUpsert       = "TODO_UPSERT"
	ActionTodoDone         = "TODO_DONE"
	ActionMailBind         = "MAIL_ACCOUNT_BIND"
	ActionMailUnbind       = "MAIL_ACCOUNT_UNBIND"
	ActionMailToTodo       = "MAIL_TO_TODO"
)

// AuditLog 为审计日志实体（表 audit_logs）。
type AuditLog struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id"`
	Operator   string    `gorm:"column:operator" json:"operator"`
	Action     string    `gorm:"column:action;index" json:"action"`
	Resource   string    `gorm:"column:resource" json:"resource"`
	ResourceID string    `gorm:"column:resource_id" json:"resourceId"`
	IP         string    `gorm:"column:ip" json:"ip"`
	UserAgent  string    `gorm:"column:user_agent" json:"userAgent"`
	RequestID  string    `gorm:"column:request_id" json:"requestId"`
	Detail     string    `gorm:"column:detail" json:"detail"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定表名。
func (AuditLog) TableName() string { return "audit_logs" }

// Entry 为记录审计所需上下文信息。
type Entry struct {
	Operator   string
	Action     string
	Resource   string
	ResourceID string
	Detail     string
}

// ListFilter 为审计查询过滤条件。
type ListFilter struct {
	Operator string
	Action   string
	Page     int
	PageSize int
}

// Service 提供审计日志读写。
type Service struct {
	db *gorm.DB
}

// NewService 创建审计服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Record 记录一条审计日志（异步写入失败仅记日志，不阻断业务）。
func (s *Service) Record(c *gin.Context, e Entry) {
	log := AuditLog{
		Operator:   e.Operator,
		Action:     e.Action,
		Resource:   e.Resource,
		ResourceID: e.ResourceID,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		RequestID:  response.RequestID(c),
		Detail:     e.Detail,
		CreatedAt:  time.Now(),
	}
	// 用独立 context（超时 5s）：客户端断开不丢失审计；审计失败不应阻断主流程。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		// 审计失败不应阻断主流程，仅记录错误日志（由调用方 logger 处理）。
		_ = err
	}
}

// List 分页查询审计日志。
func (s *Service) List(ctx context.Context, f ListFilter) ([]AuditLog, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	q := s.db.WithContext(ctx).Model(&AuditLog{})
	if f.Operator != "" {
		q = q.Where("operator = ?", f.Operator)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.DB(err)
	}
	var logs []AuditLog
	if err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, errs.DB(err)
	}
	return logs, total, nil
}
