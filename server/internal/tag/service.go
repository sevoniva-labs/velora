package tag

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Service 提供标签 CRUD。
type Service struct {
	db *gorm.DB
}

// NewService 创建标签服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// List 返回全部标签。
func (s *Service) List(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	if err := s.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&tags).Error; err != nil {
		return nil, errs.DB(err)
	}
	return tags, nil
}

// Get 查询单个标签。
func (s *Service) Get(ctx context.Context, id uint64) (*Tag, error) {
	var t Tag
	if err := s.db.WithContext(ctx).First(&t, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.NotFound(errs.CodeTagNotFound, "标签不存在")
		}
		return nil, errs.DB(err)
	}
	return &t, nil
}

// Create 创建标签。
func (s *Service) Create(ctx context.Context, in Input) (*Tag, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	if err := s.ensureCodeUnique(ctx, strings.TrimSpace(in.Code), 0); err != nil {
		return nil, err
	}
	t := &Tag{
		Code: strings.TrimSpace(in.Code),
		Name: strings.TrimSpace(in.Name),
		Sort: in.Sort,
	}
	if err := s.db.WithContext(ctx).Create(t).Error; err != nil {
		return nil, errs.DB(err)
	}
	return t, nil
}

// Update 更新标签。
func (s *Service) Update(ctx context.Context, id uint64, in Input) (*Tag, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	t, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCodeUnique(ctx, strings.TrimSpace(in.Code), id); err != nil {
		return nil, err
	}
	t.Code = strings.TrimSpace(in.Code)
	t.Name = strings.TrimSpace(in.Name)
	t.Sort = in.Sort
	if err := s.db.WithContext(ctx).Save(t).Error; err != nil {
		return nil, errs.DB(err)
	}
	return t, nil
}

// Delete 删除标签（关联关系级联删除）。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM application_tag_relations WHERE tag_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&Tag{}, id).Error
	}); err != nil {
		return errs.DB(err)
	}
	return nil
}

func (s *Service) ensureCodeUnique(ctx context.Context, code string, excludeID uint64) error {
	var count int64
	q := s.db.WithContext(ctx).Model(&Tag{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return errs.DB(err)
	}
	if count > 0 {
		return errs.New(errs.CodeTagCodeExists, 409, "标签编码已存在")
	}
	return nil
}

func validate(in Input) error {
	if strings.TrimSpace(in.Code) == "" {
		return errs.InvalidParam("标签编码不能为空")
	}
	if strings.TrimSpace(in.Name) == "" {
		return errs.InvalidParam("标签名称不能为空")
	}
	return nil
}
