package application

import (
	"context"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Repository 封装 applications 及关联表的数据库访问。
type Repository struct {
	db *gorm.DB
}

// NewRepository 创建应用仓库。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// List 分页查询应用（含分类/标签/策略预加载）。
func (r *Repository) List(ctx context.Context, q *gorm.DB) ([]Application, error) {
	var apps []Application
	err := q.
		Preload("Category").
		Preload("Tags").
		Preload("Policies").
		Find(&apps).Error
	if err != nil {
		return nil, errs.DB(err)
	}
	return apps, nil
}

// Get 查询单个应用（含关联）。
func (r *Repository) Get(ctx context.Context, id uint64) (*Application, error) {
	var app Application
	err := r.db.WithContext(ctx).
		Preload("Category").
		Preload("Tags").
		Preload("Policies").
		First(&app, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.NotFound(errs.CodeApplicationNotFound, "应用不存在")
		}
		return nil, errs.DB(err)
	}
	return &app, nil
}

// GetByCode 按编码查询（唯一性校验用）。
func (r *Repository) GetByCode(ctx context.Context, code string) (*Application, error) {
	var app Application
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&app).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errs.DB(err)
	}
	return &app, nil
}

// GetByCasdoorClientID 按 Casdoor 客户端 ID 查询（同步匹配用）。
func (r *Repository) GetByCasdoorClientID(ctx context.Context, clientID string) (*Application, error) {
	var app Application
	err := r.db.WithContext(ctx).Where("casdoor_client_id = ?", clientID).First(&app).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, errs.DB(err)
	}
	return &app, nil
}

// ListAll 查询全部应用（无分页，同步对账用）。
func (r *Repository) ListAll(ctx context.Context) ([]Application, error) {
	var apps []Application
	if err := r.db.WithContext(ctx).Find(&apps).Error; err != nil {
		return nil, errs.DB(err)
	}
	return apps, nil
}

// Create 创建应用。
func (r *Repository) Create(ctx context.Context, app *Application) error {
	if err := r.db.WithContext(ctx).Create(app).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

// Update 更新应用基础字段。
func (r *Repository) Update(ctx context.Context, app *Application) error {
	if err := r.db.WithContext(ctx).Save(app).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

// Delete 删除应用（级联删除标签关系/策略/收藏/访问记录）。
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("application_id = ?", id).Delete(&AccessPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM application_tag_relations WHERE application_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM application_favorites WHERE application_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM application_visits WHERE application_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&Application{}, id).Error
	})
	if err != nil {
		return errs.DB(err)
	}
	return nil
}

// ReplaceTags 全量替换应用标签关系。
func (r *Repository) ReplaceTags(ctx context.Context, appID uint64, tagIDs []uint64) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM application_tag_relations WHERE application_id = ?", appID).Error; err != nil {
			return err
		}
		for _, tid := range tagIDs {
			if err := tx.Exec(
				"INSERT INTO application_tag_relations (application_id, tag_id) VALUES (?, ?)",
				appID, tid,
			).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return errs.DB(err)
	}
	return nil
}

// ReplacePolicies 全量替换访问策略（记录审计由 handler 负责）。
func (r *Repository) ReplacePolicies(ctx context.Context, appID uint64, policies []AccessPolicy) error {
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("application_id = ?", appID).Delete(&AccessPolicy{}).Error; err != nil {
			return err
		}
		for i := range policies {
			policies[i].ID = 0
			policies[i].ApplicationID = appID
			if err := tx.Create(&policies[i]).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return errs.DB(err)
	}
	return nil
}
