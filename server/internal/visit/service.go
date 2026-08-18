// Package visit 提供应用访问记录（最近使用 / 热门统计基础）。
package visit

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Visit 为访问统计实体（表 application_visits）。
type Visit struct {
	UserID        string    `gorm:"column:user_id;primaryKey" json:"userId"`
	ApplicationID uint64    `gorm:"column:application_id;primaryKey" json:"applicationId"`
	VisitCount    int64     `gorm:"column:visit_count" json:"visitCount"`
	LastVisitedAt time.Time `gorm:"column:last_visited_at" json:"lastVisitedAt"`
}

// TableName 指定表名。
func (Visit) TableName() string { return "application_visits" }

// Service 提供访问记录写入。
type Service struct {
	db *gorm.DB
}

// NewService 创建访问记录服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// RecordLaunch 记录一次应用启动（UPSERT：计数 +1，更新最近访问时间）。
func (s *Service) RecordLaunch(ctx context.Context, userID string, appID uint64) error {
	err := s.db.WithContext(ctx).Exec(`
		INSERT INTO application_visits (user_id, application_id, visit_count, last_visited_at)
		VALUES (?, ?, 1, now())
		ON CONFLICT (user_id, application_id)
		DO UPDATE SET visit_count = application_visits.visit_count + 1,
		              last_visited_at = now()`,
		userID, appID,
	).Error
	if err != nil {
		return errs.DB(err)
	}
	return nil
}
