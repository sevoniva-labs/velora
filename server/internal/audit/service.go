// Package audit 提供审计日志记录与查询。
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// 审计动作常量（与前端标签映射保持一致）。
const (
	ActionLogin            = "LOGIN"
	ActionLogout           = "LOGOUT"
	ActionLoginFailed      = "LOGIN_FAILED" // 登录失败（凭据错误/锁定拒绝）——安全告警依据
	ActionAppCreate        = "APPLICATION_CREATE"
	ActionAppUpdate        = "APPLICATION_UPDATE"
	ActionAppDelete        = "APPLICATION_DELETE"
	ActionAppLaunch        = "APPLICATION_LAUNCH"
	ActionFavoriteAdd      = "FAVORITE_ADD"
	ActionFavoriteRemove   = "FAVORITE_REMOVE"
	ActionPermissionChange = "PERMISSION_CHANGE"
	ActionCategoryCreate   = "CATEGORY_CREATE"
	ActionCategoryUpdate   = "CATEGORY_UPDATE"
	ActionCategoryDelete   = "CATEGORY_DELETE"
	ActionTagCreate        = "TAG_CREATE"
	ActionTagUpdate        = "TAG_UPDATE"
	ActionTagDelete        = "TAG_DELETE"
	ActionSettingUpdate    = "SETTING_UPDATE"
	ActionTodoUpsert       = "TODO_UPSERT"
	ActionTodoDone         = "TODO_DONE"
	ActionMailBind         = "MAIL_ACCOUNT_BIND"
	ActionMailUnbind       = "MAIL_ACCOUNT_UNBIND"
	ActionOIDCAuthorize    = "OIDC_AUTHORIZE"
	ActionOIDCToken        = "OIDC_TOKEN"
	ActionMailToTodo       = "MAIL_TO_TODO"
	ActionSessionRevoke    = "SESSION_REVOKE"
)

// AuditLog 为审计日志实体（表 audit_logs）。
// prev_hash / hash 构成防篡改链（Phase C5）：hash = SHA256(prev_hash|action|resource|resource_id|operator|ip|detail|created_at)。
type AuditLog struct {
	ID         uint64    `gorm:"column:id;primaryKey" json:"id"`
	Operator   string    `gorm:"column:operator" json:"operator"`
	Action     string    `gorm:"column:action;index" json:"action"`
	Resource   string    `gorm:"column:resource" json:"resource"`
	ResourceID string    `gorm:"column:resource_id" json:"resourceId"`
	IP         string    `gorm:"column:ip" json:"ip"`
	UserAgent  string    `gorm:"column:user_agent" json:"userAgent"`
	RequestID  string    `gorm:"column:request_id" json:"requestId"`
	Detail     string    `gorm:"column:detail" json:"detail"`
	PrevHash   string    `gorm:"column:prev_hash" json:"prevHash"`
	Hash       string    `gorm:"column:hash" json:"hash"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

// TableName 指定表名。
func (AuditLog) TableName() string { return "audit_logs" }

// Entry 为记录审计所需上下文信息。
type Entry struct {
	Operator   string
	Action     string
	Resource   string
	ResourceID string
	Detail     string
}

// chainHash 计算审计链哈希。
// 时间统一截断到微秒（PostgreSQL TIMESTAMPTZ 精度），避免写入时 time.Now() 纳秒
// 与读回时微秒不一致导致校验失败。
func chainHash(prevHash, action, resource, resourceID, operator, ip, detail string, createdAt time.Time) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%s|%s", prevHash, action, resource, resourceID, operator, ip, detail, formatChainTime(createdAt))
	return hex.EncodeToString(h.Sum(nil))
}

// formatChainTime 格式化为 UTC 微秒精度的 RFC3339 串（与 DB 存储精度一致）。
func formatChainTime(t time.Time) string {
	return t.UTC().Truncate(time.Microsecond).Format("2006-01-02T15:04:05.000000Z")
}

// ListFilter 为审计查询过滤条件。
type ListFilter struct {
	Operator string
	Action   string
	Page     int
	PageSize int
}

// Service 提供审计日志读写。
type Service struct {
	db *gorm.DB
}

// NewService 创建审计服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Record 记录一条审计日志（异步写入失败仅记日志，不阻断业务）。
func (s *Service) Record(c *gin.Context, e Entry) {
	now := time.Now()
	// 防篡改链：取最新一条的 hash 作为 prev_hash（空库时 prev=""）。
	prev := s.lastHash(context.Background())
	log := AuditLog{
		Operator:   e.Operator,
		Action:     e.Action,
		Resource:   e.Resource,
		ResourceID: e.ResourceID,
		IP:         c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		RequestID:  response.RequestID(c),
		Detail:     e.Detail,
		PrevHash:   prev,
		CreatedAt:  now,
	}
	log.Hash = chainHash(prev, e.Action, e.Resource, e.ResourceID, e.Operator, log.IP, e.Detail, now)
	// 用独立 context（超时 5s）：客户端断开不丢失审计；审计失败不应阻断主流程。
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		// 审计失败不应阻断主流程，仅记录错误日志（由调用方 logger 处理）。
		metrics.Emit("velora_audit_write_failure_total")
		_ = err
	}
}

// lastHash 返回最新审计记录 hash（防篡改链前驱）。
func (s *Service) lastHash(ctx context.Context) string {
	var h string
	_ = s.db.WithContext(ctx).Model(&AuditLog{}).
		Where("hash <> ''").
		Order("id DESC").Limit(1).Pluck("hash", &h).Error
	return h
}

// RecordWithMeta 记录审计（无 gin.Context 场景：服务端任务如邮件同步/登录失败）。
func (s *Service) RecordWithMeta(ctx context.Context, e Entry, ip, userAgent, requestID string) {
	now := time.Now()
	prev := s.lastHash(ctx)
	log := AuditLog{
		Operator:   e.Operator,
		Action:     e.Action,
		Resource:   e.Resource,
		ResourceID: e.ResourceID,
		IP:         ip,
		UserAgent:  userAgent,
		RequestID:  requestID,
		Detail:     e.Detail,
		PrevHash:   prev,
		CreatedAt:  now,
	}
	log.Hash = chainHash(prev, e.Action, e.Resource, e.ResourceID, e.Operator, ip, e.Detail, now)
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.db.WithContext(ctx2).Create(&log).Error; err != nil {
		metrics.Emit("velora_audit_write_failure_total")
		_ = err
	}
}

// BackfillChain 回填历史审计记录的哈希链（幂等：仅处理 hash='' 的记录）。
// 启动时调用，确保升级前存量记录与运行时算法一致。
func (s *Service) BackfillChain(ctx context.Context) error {
	var logs []AuditLog
	if err := s.db.WithContext(ctx).Model(&AuditLog{}).
		Where("hash = ''").
		Order("id ASC").
		Find(&logs).Error; err != nil {
		return errs.DB(err)
	}
	if len(logs) == 0 {
		return nil
	}
	// 链前驱：最后一条已回填记录的 hash（若无则查最新一条）
	prev := ""
	var last string
	if err := s.db.WithContext(ctx).Model(&AuditLog{}).
		Where("hash <> ''").Order("id DESC").Limit(1).Pluck("hash", &last).Error; err == nil {
		prev = last
	}
	for _, l := range logs {
		l.PrevHash = prev
		l.Hash = chainHash(prev, l.Action, l.Resource, l.ResourceID, l.Operator, l.IP, l.Detail, l.CreatedAt)
		if err := s.db.WithContext(ctx).Model(&AuditLog{}).Where("id = ?", l.ID).
			Updates(map[string]any{"prev_hash": l.PrevHash, "hash": l.Hash}).Error; err != nil {
			return errs.DB(err)
		}
		prev = l.Hash
	}
	return nil
}

// VerifyChain 校验审计链完整性：从第 startID 条起逐条重算 hash 比对。
// 返回 (ok, 第一条不一致的记录 ID, 错误)。ok=false 且 err=nil 表示检测到篡改；
// err != nil 表示校验过程中系统错误（如数据库故障）。
func (s *Service) VerifyChain(ctx context.Context, startID uint64) (bool, uint64, error) {
	var logs []AuditLog
	q := s.db.WithContext(ctx).Model(&AuditLog{}).Order("id ASC")
	if startID > 0 {
		q = q.Where("id >= ?", startID)
	}
	if err := q.Find(&logs).Error; err != nil {
		return false, 0, errs.DB(err)
	}
	prev := ""
	for _, l := range logs {
		if l.Hash == "" {
			continue // 未回填的历史记录（升级前）
		}
		expect := chainHash(prev, l.Action, l.Resource, l.ResourceID, l.Operator, l.IP, l.Detail, l.CreatedAt)
		if l.Hash != expect {
			return false, l.ID, nil
		}
		prev = l.Hash
	}
	return true, 0, nil
}

// List 分页查询审计日志。
func (s *Service) List(ctx context.Context, f ListFilter) ([]AuditLog, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	q := s.db.WithContext(ctx).Model(&AuditLog{})
	if f.Operator != "" {
		q = q.Where("operator = ?", f.Operator)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.DB(err)
	}
	var logs []AuditLog
	if err := q.Order("created_at DESC").
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&logs).Error; err != nil {
		return nil, 0, errs.DB(err)
	}
	return logs, total, nil
}
