// Package category 提供应用分类管理。
package category

import "time"

// Category 为应用分类实体（表 application_categories）。
type Category struct {
	ID          uint64    `gorm:"column:id;primaryKey" json:"id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	Sort        int       `gorm:"column:sort" json:"sort"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Category) TableName() string { return "application_categories" }

// Input 为分类创建/更新入参。
type Input struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}
