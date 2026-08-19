// Package application 为 Velora 应用中心领域：应用 CRUD、可见性、Launch。
package application

import (
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/category"
	"github.com/sevoniva-labs/velora/server/internal/tag"
)

// SSO 接入类型（数据模型第一天预留，第一阶段实现 URL / OIDC）。
const (
	SSOTypeURL         = "URL"
	SSOTypeOIDC        = "OIDC"
	SSOTypeSAML        = "SAML"
	SSOTypeCAS         = "CAS"
	SSOTypeForwardAuth = "FORWARD_AUTH"
	// SSOTypeVeloraOIDC：应用通过 Velora 自身 OIDC Provider 登录（Phase B 核心）。
	// 与 SSOTypeOIDC 的区别：OIDC 直连 Casdoor；VELORA_OIDC 走 Velora /oidc/* 终点。
	SSOTypeVeloraOIDC = "VELORA_OIDC"
)

// 应用状态。
const (
	StatusEnabled  = "ENABLED"
	StatusDisabled = "DISABLED"
)

// 健康状态。
const (
	HealthUp      = "UP"
	HealthDown    = "DOWN"
	HealthUnknown = "UNKNOWN"
)

// Application 为应用实体（表 applications）。
type Application struct {
	ID                     uint64             `gorm:"column:id;primaryKey" json:"id"`
	Code                   string             `gorm:"column:code;uniqueIndex" json:"code"`
	Name                   string             `gorm:"column:name" json:"name"`
	Description            string             `gorm:"column:description" json:"description"`
	Keywords               string             `gorm:"column:keywords" json:"keywords"`
	Icon                   string             `gorm:"column:icon" json:"icon"`
	CategoryID             *uint64            `gorm:"column:category_id" json:"categoryId"`
	Category               *category.Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	HomeURL                string             `gorm:"column:home_url" json:"homeUrl"`
	LaunchURL              string             `gorm:"column:launch_url" json:"launchUrl"`
	SSOType                string             `gorm:"column:sso_type" json:"ssoType"`
	CasdoorApplicationName string             `gorm:"column:casdoor_application_name" json:"casdoorApplicationName"`
	CasdoorClientID        string             `gorm:"column:casdoor_client_id" json:"casdoorClientId"`
	Owner                  string             `gorm:"column:owner" json:"owner"`
	Department             string             `gorm:"column:department" json:"department"`
	Status                 string             `gorm:"column:status" json:"status"`
	Sort                   int                `gorm:"column:sort" json:"sort"`
	IsFeatured             bool               `gorm:"column:is_featured" json:"isFeatured"`
	HealthCheckEnabled     bool               `gorm:"column:health_check_enabled" json:"healthCheckEnabled"`
	HealthCheckURL         string             `gorm:"column:health_check_url" json:"healthCheckUrl"`
	CreatedAt              time.Time          `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              time.Time          `gorm:"column:updated_at" json:"updatedAt"`
	CreatedBy              string             `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy              string             `gorm:"column:updated_by" json:"updatedBy"`

	Tags     []tag.Tag      `gorm:"many2many:application_tag_relations;joinForeignKey:ApplicationID;joinReferences:TagID" json:"tags"`
	Policies []AccessPolicy `gorm:"foreignKey:ApplicationID" json:"policies,omitempty"`
}

// TableName 指定表名。
func (Application) TableName() string { return "applications" }

// AccessPolicy 为应用访问策略（表 application_access_policies）。
type AccessPolicy struct {
	ID            uint64    `gorm:"column:id;primaryKey" json:"id"`
	ApplicationID uint64    `gorm:"column:application_id;index" json:"applicationId"`
	PolicyType    string    `gorm:"column:policy_type" json:"policyType"`
	Value         string    `gorm:"column:value" json:"value"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (AccessPolicy) TableName() string { return "application_access_policies" }

// PolicyType 为策略类型常量。
const (
	PolicyTypeEveryone     = "EVERYONE"
	PolicyTypeOrganization = "ORGANIZATION"
	PolicyTypeRole         = "ROLE"
	PolicyTypeGroup        = "GROUP"
	PolicyTypeUser         = "USER"
)

// EnsureGORM 仅用于确保 gorm 关联类型被编译期引用。
var _ = gorm.ErrRecordNotFound
