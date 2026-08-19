package auth

import (
	"encoding/json"
	"time"

	"gorm.io/gorm/clause"
)

// ServerSession 为 sessions 表记录（Phase B6 服务端会话）。
type ServerSession struct {
	ID           uint64     `gorm:"column:id;primaryKey" json:"-"`
	SessionID    string     `gorm:"column:session_id;uniqueIndex" json:"-"`
	UserID       string     `gorm:"column:user_id" json:"userId"`
	Username     string     `gorm:"column:username" json:"username"`
	DisplayName  string     `gorm:"column:display_name" json:"displayName"`
	Email        string     `gorm:"column:email" json:"email"`
	Avatar       string     `gorm:"column:avatar" json:"avatar"`
	Organization string     `gorm:"column:organization" json:"organization"`
	RolesRaw     string     `gorm:"column:roles" json:"-"`
	GroupsRaw    string     `gorm:"column:groups" json:"-"`
	UserAgent    string     `gorm:"column:user_agent" json:"userAgent"`
	IP           string     `gorm:"column:ip" json:"ip"`
	IssuedAt     time.Time  `gorm:"column:issued_at" json:"issuedAt"`
	ExpiresAt    time.Time  `gorm:"column:expires_at" json:"expiresAt"`
	LastActiveAt time.Time  `gorm:"column:last_active_at" json:"lastActiveAt"`
	RevokedAt    *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"-"`
}

// TableName 指定表名。
func (ServerSession) TableName() string { return "sessions" }

// Roles 解析角色 JSON。
func (r *ServerSession) Roles() []string {
	var out []string
	_ = json.Unmarshal([]byte(r.RolesRaw), &out)
	return out
}

// Groups 解析分组 JSON。
func (r *ServerSession) Groups() []string {
	var out []string
	_ = json.Unmarshal([]byte(r.GroupsRaw), &out)
	return out
}

// ToCurrentUser 派生当前用户视图（供 OIDC token 用户快照使用）。
func (r *ServerSession) ToCurrentUser() *CurrentUser {
	return &CurrentUser{
		ID:           r.UserID,
		Username:     r.Username,
		DisplayName:  r.DisplayName,
		Email:        r.Email,
		Avatar:       r.Avatar,
		Organization: r.Organization,
		Roles:        r.Roles(),
		Groups:       r.Groups(),
	}
}

// clauseOnConflictSessionID 供 persist 使用：session_id 冲突时更新（续期）。
var clauseOnConflictSessionID = clause.OnConflict{
	Columns: []clause.Column{{Name: "session_id"}},
	DoUpdates: clause.AssignmentColumns([]string{
		"username", "display_name", "email", "avatar", "organization",
		"roles", "groups", "expires_at", "last_active_at",
	}),
}

// ---------- 会话吊销 API（Phase C1 复用） ----------

// Revoke 吊销指定会话（按 SID）。
func (s *SessionStore) Revoke(sid string) error {
	if s.db == nil {
		return nil // 无服务端会话：无操作
	}
	now := time.Now()
	return s.db.Model(&ServerSession{}).
		Where("session_id = ? AND revoked_at IS NULL", sid).
		Update("revoked_at", now).Error
}

// RevokeAllForUser 吊销用户全部会话（改密/强制下线）。
func (s *SessionStore) RevokeAllForUser(userID string) error {
	if s.db == nil {
		return nil
	}
	now := time.Now()
	return s.db.Model(&ServerSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// ListForUser 列出用户全部会话（设备列表，含已吊销）。
func (s *SessionStore) ListForUser(userID string) ([]ServerSession, error) {
	if s.db == nil {
		return nil, nil
	}
	var list []ServerSession
	err := s.db.Where("user_id = ?", userID).Order("last_active_at DESC").Find(&list).Error
	return list, err
}

// CountActiveForUser 统计用户活跃会话数（安全策略：限制同时登录设备数）。
func (s *SessionStore) CountActiveForUser(userID string) (int64, error) {
	if s.db == nil {
		return 0, nil
	}
	var n int64
	err := s.db.Model(&ServerSession{}).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID, time.Now()).
		Count(&n).Error
	return n, err
}

// PruneExpired 清理过期会话（后台任务调用）。
func (s *SessionStore) PruneExpired(before time.Time) (int64, error) {
	if s.db == nil {
		return 0, nil
	}
	res := s.db.Where("expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)", before, before).
		Delete(&ServerSession{})
	return res.RowsAffected, res.Error
}
