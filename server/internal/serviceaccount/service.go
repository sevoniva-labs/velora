// Package serviceaccount 提供集成令牌（service account）鉴权（Phase D3）。
//
// 外部系统通过 `Authorization: Bearer <token>` 调用集成端点（如推送待办），
// 与用户会话解耦；token 明文仅创建时返回一次，库中存 SHA-256 哈希。
package serviceaccount

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/permission"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// Scope 集成令牌权限 scope。
const (
	ScopeTodoWrite = "todo:write"
)

// IntegrationToken 为集成令牌实体（表 integration_tokens）。
type IntegrationToken struct {
	ID         uint64     `gorm:"column:id;primaryKey" json:"id"`
	Name       string     `gorm:"column:name" json:"name"`
	TokenHash  string     `gorm:"column:token_hash;uniqueIndex" json:"-"`
	ScopesRaw  string     `gorm:"column:scopes" json:"-"`
	CreatedBy  string     `gorm:"column:created_by" json:"createdBy"`
	ExpiresAt  *time.Time `gorm:"column:expires_at" json:"expiresAt"`
	Revoked    bool       `gorm:"column:revoked" json:"revoked"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定表名。
func (IntegrationToken) TableName() string { return "integration_tokens" }

// Scopes 返回权限 scope 列表。
func (t *IntegrationToken) Scopes() []string {
	if t.ScopesRaw == "" {
		return nil
	}
	return strings.Split(t.ScopesRaw, ",")
}

// HasScope 判断是否具备指定 scope。
func (t *IntegrationToken) HasScope(scope string) bool {
	for _, s := range t.Scopes() {
		if s == scope {
			return true
		}
	}
	return false
}

// Service 提供令牌管理。
type Service struct {
	db *gorm.DB
}

// NewService 创建令牌服务。
func NewService(db *gorm.DB) *Service { return &Service{db: db} }

// Create 创建令牌，返回明文 token（仅此一次）。
func (s *Service) Create(ctx context.Context, name, createdBy string, scopes []string, expiresAt *time.Time) (token string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", errs.Internal("生成令牌失败", err)
	}
	plain := "velora_" + hex.EncodeToString(raw)
	rec := IntegrationToken{
		Name:      name,
		TokenHash: hashToken(plain),
		ScopesRaw: strings.Join(scopes, ","),
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return "", errs.DB(err)
	}
	return plain, nil
}

// Revoke 吊销令牌。
func (s *Service) Revoke(ctx context.Context, id uint64) error {
	res := s.db.WithContext(ctx).Model(&IntegrationToken{}).
		Where("id = ? AND revoked = ?", id, false).
		Update("revoked", true)
	if res.Error != nil {
		return errs.DB(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, 404, "令牌不存在或已吊销")
	}
	return nil
}

// List 列出全部令牌（不含明文）。
func (s *Service) List(ctx context.Context) ([]IntegrationToken, error) {
	var list []IntegrationToken
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, errs.DB(err)
	}
	return list, nil
}

// Authenticate 校验 Bearer token 并返回对应令牌（有效且未吊销、未过期）。
func (s *Service) Authenticate(ctx context.Context, bearer string) (*IntegrationToken, error) {
	token := strings.TrimSpace(bearer)
	if !strings.HasPrefix(token, "velora_") {
		return nil, errors.New("令牌格式无效")
	}
	rec := &IntegrationToken{}
	err := s.db.WithContext(ctx).Where("token_hash = ? AND revoked = ?", hashToken(token), false).First(rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("令牌无效或已吊销")
		}
		return nil, errs.DB(err)
	}
	if rec.ExpiresAt != nil && time.Now().After(*rec.ExpiresAt) {
		return nil, errors.New("令牌已过期")
	}
	// 更新最近使用时间（尽力而为，不阻断）
	now := time.Now()
	_ = s.db.WithContext(ctx).Model(rec).Update("last_used_at", now).Error
	rec.LastUsedAt = &now
	return rec, nil
}

// hashToken SHA-256 十六进制。
func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// --- HTTP ---

// Handler 提供令牌管理端点（管理员）。
type Handler struct {
	svc       *Service
	adminRole string
}

// NewHandler 创建令牌管理 Handler。
func NewHandler(svc *Service, adminRole string) *Handler {
	return &Handler{svc: svc, adminRole: adminRole}
}

// Register 注册路由（管理员）。
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/admin/integration-tokens")
	g.Use(permission.AdminRequired(h.adminRole))
	g.GET("", h.list)
	g.POST("", h.create)
	g.DELETE("/:id", h.revoke)
}

func (h *Handler) list(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

func (h *Handler) create(c *gin.Context) {
	actor := ""
	if u, err := currentUser(c); err == nil {
		actor = u.Username
	}
	var body struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, errs.InvalidParam("请求体格式错误"))
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		response.Error(c, errs.InvalidParam("令牌名称必填"))
		return
	}
	if len(body.Scopes) == 0 {
		response.Error(c, errs.InvalidParam("至少选择一个权限 scope"))
		return
	}
	token, err := h.svc.Create(c.Request.Context(), body.Name, actor, body.Scopes, body.ExpiresAt)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"token": token, "message": "令牌明文仅显示一次，请立即保存"})
}

func (h *Handler) revoke(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errs.InvalidParam("令牌 ID 无效"))
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

// currentUser 从 gin 上下文解析当前用户（用于记录创建者；无会话时为匿名）。
func currentUser(c *gin.Context) (*auth.CurrentUser, error) {
	return auth.RequireUser(c)
}

// BearerAuth 中间件：从 Authorization: Bearer 校验集成令牌并注入上下文。
// 失败返回 401；成功时 c.Set("integration_token", *IntegrationToken)。
func (s *Service) BearerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.ErrorWith(c, http.StatusUnauthorized, errs.CodeUnauthorized, "缺少 Bearer 令牌")
			c.Abort()
			return
		}
		rec, err := s.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			response.ErrorWith(c, http.StatusUnauthorized, errs.CodeUnauthorized, err.Error())
			c.Abort()
			return
		}
		c.Set("integration_token", rec)
		c.Next()
	}
}

// RequireScope 校验当前集成令牌具备指定 scope（需先经 BearerAuth）。
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get("integration_token")
		if !ok {
			response.ErrorWith(c, http.StatusUnauthorized, errs.CodeUnauthorized, "需要集成令牌")
			c.Abort()
			return
		}
		rec, ok := v.(*IntegrationToken)
		if !ok || !rec.HasScope(scope) {
			response.ErrorWith(c, http.StatusForbidden, errs.CodeForbidden, "令牌无权执行该操作（缺少 scope: "+scope+"）")
			c.Abort()
			return
		}
		c.Next()
	}
}
