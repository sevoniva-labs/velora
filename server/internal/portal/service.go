// Package portal 提供门户设置与管理员概览统计。
package portal

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Setting 为门户设置实体（表 portal_settings）。
type Setting struct {
	Key       string    `gorm:"column:key;primaryKey" json:"key"`
	Value     string    `gorm:"column:value" json:"value"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Setting) TableName() string { return "portal_settings" }

// Service 提供门户设置读写。
type Service struct {
	db *gorm.DB
}

// NewService 创建门户设置服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Get 读取单个设置（不存在返回空串）。
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	var st Setting
	err := s.db.WithContext(ctx).First(&st, "key = ?", key).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", errs.DB(err)
	}
	return st.Value, nil
}

// All 返回全部设置。
func (s *Service) All(ctx context.Context) ([]Setting, error) {
	var list []Setting
	if err := s.db.WithContext(ctx).Order("key ASC").Find(&list).Error; err != nil {
		return nil, errs.DB(err)
	}
	return list, nil
}

// Set 写入设置（UPSERT）。
func (s *Service) Set(ctx context.Context, key, value string) error {
	if key == "" {
		return errs.InvalidParam("设置键不能为空")
	}
	err := s.db.WithContext(ctx).Exec(`
		INSERT INTO portal_settings (key, value, updated_at)
		VALUES (?, ?, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, value,
	).Error
	if err != nil {
		return errs.DB(err)
	}
	return nil
}

// DashboardStats 为管理员概览统计。
type DashboardStats struct {
	ApplicationCount int64 `json:"applicationCount"`
	CategoryCount    int64 `json:"categoryCount"`
	TagCount         int64 `json:"tagCount"`
	FavoriteCount    int64 `json:"favoriteCount"`
	TotalLaunches    int64 `json:"totalLaunches"`
	EnabledAppCount  int64 `json:"enabledAppCount"`
	DisabledAppCount int64 `json:"disabledAppCount"`
}

// Dashboard 返回管理员概览统计。
func (s *Service) Dashboard(ctx context.Context) (*DashboardStats, error) {
	var stats DashboardStats
	counts := []struct {
		table  string
		target *int64
	}{
		{"applications", &stats.ApplicationCount},
		{"application_categories", &stats.CategoryCount},
		{"application_tags", &stats.TagCount},
		{"application_favorites", &stats.FavoriteCount},
	}
	for _, c := range counts {
		if err := s.db.WithContext(ctx).Table(c.table).Count(c.target).Error; err != nil {
			return nil, errs.DB(err)
		}
	}
	if err := s.db.WithContext(ctx).Table("application_visits").
		Select("COALESCE(SUM(visit_count), 0)").Scan(&stats.TotalLaunches).Error; err != nil {
		return nil, errs.DB(err)
	}
	if err := s.db.WithContext(ctx).Table("applications").
		Where("status = 'ENABLED'").Count(&stats.EnabledAppCount).Error; err != nil {
		return nil, errs.DB(err)
	}
	if err := s.db.WithContext(ctx).Table("applications").
		Where("status = 'DISABLED'").Count(&stats.DisabledAppCount).Error; err != nil {
		return nil, errs.DB(err)
	}
	return &stats, nil
}
