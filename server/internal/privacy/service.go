// Package privacy 提供用户数据导出（Phase D5，GDPR/个保法合规）。
//
// 管理员可导出指定用户的 Velora 侧全量数据：收藏、访问记录、待办、
// 邮件元数据、审计操作记录，打包为 JSON 附件下载，便于用户行使
// 数据可携带权 / 删除前的归档留证。
package privacy

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Export 为用户数据导出结果。
type Export struct {
	UserID      string        `json:"userId"`
	GeneratedAt time.Time     `json:"generatedAt"`
	Favorites   []favoriteRow `json:"favorites"`
	Visits      []visitRow    `json:"visits"`
	Todos       []todoRow     `json:"todos"`
	MailMeta    []mailRow     `json:"mailMeta"`
	AuditLogs   []auditRow    `json:"auditLogs"`
}

type favoriteRow struct {
	ApplicationID uint64    `json:"applicationId"`
	CreatedAt     time.Time `json:"createdAt"`
}

type visitRow struct {
	ApplicationID uint64    `json:"applicationId"`
	LastVisitedAt time.Time `json:"lastVisitedAt"`
	Count         int64     `json:"count"`
}

type todoRow struct {
	ID           uint64     `json:"id"`
	Title        string     `json:"title"`
	Kind         string     `json:"kind"`
	SourceSystem string     `json:"sourceSystem"`
	SourceID     string     `json:"sourceId"`
	Priority     string     `json:"priority"`
	Status       string     `json:"status"`
	DueAt        *time.Time `json:"dueAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type mailRow struct {
	Folder        string     `json:"folder"`
	Subject       string     `json:"subject"`
	FromAddress   string     `json:"fromAddress"`
	FromName      string     `json:"fromName"`
	ToAddresses   string     `json:"toAddresses"`
	ReceivedAt    *time.Time `json:"receivedAt"`
	IsRead        bool       `json:"isRead"`
	IsStarred     bool       `json:"isStarred"`
	HasAttachment bool       `json:"hasAttachment"`
}

type auditRow struct {
	ID         uint64    `json:"id"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resourceId"`
	IP         string    `json:"ip"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Service 提供数据导出查询。
type Service struct {
	db *gorm.DB
}

// NewService 创建导出服务。
func NewService(db *gorm.DB) *Service { return &Service{db: db} }

// ExportUser 聚合用户数据。userID 同时匹配主键列与审计 operator 列。
func (s *Service) ExportUser(ctx context.Context, userID string) (*Export, error) {
	if userID == "" {
		return nil, errs.InvalidParam("用户 ID 必填")
	}
	out := &Export{UserID: userID, GeneratedAt: time.Now().UTC()}

	// 收藏
	if err := s.db.WithContext(ctx).Table("application_favorites").
		Select("application_id", "created_at").
		Where("user_id = ?", userID).Order("created_at").Find(&out.Favorites).Error; err != nil {
		return nil, errs.DB(err)
	}
	// 访问记录（含累计次数）
	if err := s.db.WithContext(ctx).Table("application_visits").
		Select("application_id", "last_visited_at", "visit_count AS count").
		Where("user_id = ?", userID).Order("last_visited_at DESC").Find(&out.Visits).Error; err != nil {
		return nil, errs.DB(err)
	}
	// 待办
	if err := s.db.WithContext(ctx).Table("todos").
		Select("id", "title", "kind", "source_system", "source_id", "priority", "status", "due_at", "created_at").
		Where("user_id = ?", userID).Order("created_at DESC").Find(&out.Todos).Error; err != nil {
		return nil, errs.DB(err)
	}
	// 邮件元数据（不含正文，控制导出体积；正文属邮件服务自身范畴）
	if err := s.db.WithContext(ctx).Table("mail_messages").
		Select("folder", "subject", "from_address", "from_name", "to_addresses", "received_at", "is_read", "is_starred", "has_attachment").
		Where("user_id = ?", userID).Order("received_at DESC NULLS LAST").Find(&out.MailMeta).Error; err != nil {
		return nil, errs.DB(err)
	}
	// 审计操作记录（该用户作为操作者的审计链条目）
	if err := s.db.WithContext(ctx).Table("audit_logs").
		Select("id", "action", "resource", "resource_id", "ip", "detail", "created_at").
		Where("operator = ?", userID).Order("id DESC").Limit(5000).Find(&out.AuditLogs).Error; err != nil {
		return nil, errs.DB(err)
	}

	return out, nil
}

// MarshalJSON 序列化（便于 handler 直接写附件）。
func (e *Export) MarshalJSON() ([]byte, error) {
	type alias Export
	return json.Marshal((*alias)(e))
}
