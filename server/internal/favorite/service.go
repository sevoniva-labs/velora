// Package favorite 提供应用收藏能力。
package favorite

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Favorite 为收藏实体（表 application_favorites）。
type Favorite struct {
	UserID        string    `gorm:"column:user_id;primaryKey" json:"userId"`
	ApplicationID uint64    `gorm:"column:application_id;primaryKey" json:"applicationId"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定表名。
func (Favorite) TableName() string { return "application_favorites" }

// Service 提供收藏读写。
type Service struct {
	db *gorm.DB
}

// NewService 创建收藏服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Add 添加收藏（幂等）。
func (s *Service) Add(ctx context.Context, userID string, appID uint64) error {
	fav := Favorite{UserID: userID, ApplicationID: appID, CreatedAt: time.Now()}
	if err := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&fav).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

// Remove 取消收藏（不存在视为成功，幂等）。
func (s *Service) Remove(ctx context.Context, userID string, appID uint64) error {
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Delete(&Favorite{}).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

// IsFavorited 查询是否已收藏。
func (s *Service) IsFavorited(ctx context.Context, userID string, appID uint64) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&Favorite{}).
		Where("user_id = ? AND application_id = ?", userID, appID).
		Count(&count).Error; err != nil {
		return false, errs.DB(err)
	}
	return count > 0, nil
}
