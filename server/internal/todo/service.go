// Package todo 提供待办中心能力：门户内置待办，并开放 API 供外部系统
// （OA / 运维工单 / 审批流等）推送集成，以来源单据为幂等键避免重复。
package todo

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// 优先级取值（与前端展示映射一致）。
const (
	PriorityUrgent = "urgent"
	PriorityHigh   = "high"
	PriorityMid    = "mid"
	PriorityLow    = "low"
)

// 状态取值。
const (
	StatusOpen = "open"
	StatusDone = "done"
)

// 待办类型（Tab 维度）：mail 邮件 | approval 审批 | devops 研发 | ops 运维 | project 项目 | hr 人事 | other 其他。
const (
	KindMail     = "mail"
	KindApproval = "approval"
	KindDevOps   = "devops"
	KindOps      = "ops"
	KindProject  = "project"
	KindHR       = "hr"
	KindOther    = "other"
)

// Todo 为待办实体（表 todos）。
type Todo struct {
	ID           uint64     `gorm:"column:id;primaryKey" json:"id"`
	UserID       string     `gorm:"column:user_id" json:"userId"`
	Title        string     `gorm:"column:title" json:"title"`
	Kind         string     `gorm:"column:kind" json:"kind"`
	SourceSystem string     `gorm:"column:source_system" json:"sourceSystem"`
	SourceLabel  string     `gorm:"column:source_label" json:"sourceLabel"`
	SourceID     string     `gorm:"column:source_id" json:"sourceId"`
	Priority     string     `gorm:"column:priority" json:"priority"`
	URL          string     `gorm:"column:url" json:"url"`
	DueAt        *time.Time `gorm:"column:due_at" json:"dueAt"`
	Status       string     `gorm:"column:status" json:"status"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Todo) TableName() string { return "todos" }

// ValidPriority 校验优先级取值。
func ValidPriority(p string) bool {
	switch p {
	case PriorityUrgent, PriorityHigh, PriorityMid, PriorityLow:
		return true
	}
	return false
}

// ValidKind 校验待办类型取值。
func ValidKind(k string) bool {
	switch k {
	case KindMail, KindApproval, KindDevOps, KindOps, KindProject, KindHR, KindOther:
		return true
	}
	return false
}

// Service 提供待办读写。
type Service struct {
	db *gorm.DB
}

// NewService 创建待办服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// List 查询用户待办：到期时间升序（无到期排后），其次创建时间倒序。
// status 支持 open / done / all（默认 open）。
func (s *Service) List(ctx context.Context, userID, status string, limit int) ([]Todo, error) {
	q := s.db.WithContext(ctx).Where("user_id = ?", userID)
	switch status {
	case StatusOpen, StatusDone:
		q = q.Where("status = ?", status)
	case "", "all":
	default:
		return nil, errs.InvalidParam("无效的待办状态")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var items []Todo
	if err := q.Order("due_at ASC NULLS LAST").Order("created_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, errs.DB(err)
	}
	return items, nil
}

// OpenCount 返回用户待处理数量。
func (s *Service) OpenCount(ctx context.Context, userID string) (int64, error) {
	var n int64
	if err := s.db.WithContext(ctx).Model(&Todo{}).
		Where("user_id = ? AND status = ?", userID, StatusOpen).
		Count(&n).Error; err != nil {
		return 0, errs.DB(err)
	}
	return n, nil
}

// Upsert 以（source_system, source_id, user_id）为幂等键写入待办：
// 同一来源单据重复推送只更新内容，不产生重复待办；已完成的待办不会被重置回 open。
func (s *Service) Upsert(ctx context.Context, t Todo) (*Todo, error) {
	now := time.Now()
	t.Status = StatusOpen
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Kind == "" {
		t.Kind = KindOther
	}
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_system"}, {Name: "source_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "source_label", "priority", "kind", "url", "due_at", "updated_at"}),
		}).
		Create(&t).Error; err != nil {
		return nil, errs.DB(err)
	}
	return &t, nil
}

// MarkDone 将本人的待办标记为完成。
func (s *Service) MarkDone(ctx context.Context, userID string, id uint64) error {
	res := s.db.WithContext(ctx).Model(&Todo{}).
		Where("id = ? AND user_id = ? AND status = ?", id, userID, StatusOpen).
		Updates(map[string]any{"status": StatusDone, "updated_at": time.Now()})
	if res.Error != nil {
		return errs.DB(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.NotFound(errs.CodeTodoNotFound, "待办不存在或已完成")
	}
	return nil
}
