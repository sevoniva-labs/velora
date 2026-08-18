// Package tag 提供应用标签管理。
package tag

import "time"

// Tag 为应用标签实体（表 application_tags）。
type Tag struct {
	ID        uint64    `gorm:"column:id;primaryKey" json:"id"`
	Code      string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name      string    `gorm:"column:name" json:"name"`
	Sort      int       `gorm:"column:sort" json:"sort"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Tag) TableName() string { return "application_tags" }

// Input 为标签创建/更新入参。
type Input struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Sort int    `json:"sort"`
}
