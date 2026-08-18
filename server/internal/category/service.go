package category

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// Service 提供分类 CRUD。
type Service struct {
	db *gorm.DB
}

// NewService 创建分类服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// List 返回全部分类（按 sort, id 排序）。
func (s *Service) List(ctx context.Context) ([]Category, error) {
	var cats []Category
	if err := s.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&cats).Error; err != nil {
		return nil, errs.DB(err)
	}
	return cats, nil
}

// Get 查询单个分类。
func (s *Service) Get(ctx context.Context, id uint64) (*Category, error) {
	var cat Category
	if err := s.db.WithContext(ctx).First(&cat, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.NotFound(errs.CodeCategoryNotFound, "分类不存在")
		}
		return nil, errs.DB(err)
	}
	return &cat, nil
}

// Create 创建分类。
func (s *Service) Create(ctx context.Context, in Input) (*Category, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	if err := s.ensureCodeUnique(ctx, strings.TrimSpace(in.Code), 0); err != nil {
		return nil, err
	}
	cat := &Category{
		Code:        strings.TrimSpace(in.Code),
		Name:        strings.TrimSpace(in.Name),
		Description: in.Description,
		Sort:        in.Sort,
	}
	if err := s.db.WithContext(ctx).Create(cat).Error; err != nil {
		return nil, errs.DB(err)
	}
	return cat, nil
}

// Update 更新分类。
func (s *Service) Update(ctx context.Context, id uint64, in Input) (*Category, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	cat, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCodeUnique(ctx, strings.TrimSpace(in.Code), id); err != nil {
		return nil, err
	}
	cat.Code = strings.TrimSpace(in.Code)
	cat.Name = strings.TrimSpace(in.Name)
	cat.Description = in.Description
	cat.Sort = in.Sort
	if err := s.db.WithContext(ctx).Save(cat).Error; err != nil {
		return nil, errs.DB(err)
	}
	return cat, nil
}

// Delete 删除分类（应用外键 ON DELETE SET NULL）。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&Category{}, id).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

func (s *Service) ensureCodeUnique(ctx context.Context, code string, excludeID uint64) error {
	var count int64
	q := s.db.WithContext(ctx).Model(&Category{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return errs.DB(err)
	}
	if count > 0 {
		return errs.New(errs.CodeCategoryCodeExists, 409, "分类编码已存在")
	}
	return nil
}

func validate(in Input) error {
	if strings.TrimSpace(in.Code) == "" {
		return errs.InvalidParam("分类编码不能为空")
	}
	if strings.TrimSpace(in.Name) == "" {
		return errs.InvalidParam("分类名称不能为空")
	}
	return nil
}
