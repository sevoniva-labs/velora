// Package mail 提供企业邮箱能力：独立 Mail 领域，通过 Provider 抽象接入
// 各类邮箱（Generic IMAP + 厂商 Profile），与 Todo 领域解耦——
// 邮件默认不进入待办，仅当用户手动"转为待办"时建立引用关联。
package mail

import "time"

// Provider 标识（仅用于记录与默认配置 Profile，业务逻辑不得依赖具体取值）。
const (
	ProviderIMAP    = "imap"
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"
	ProviderCustom  = "custom"
)

// 账号状态。
const (
	AccountStatusActive   = "active"
	AccountStatusError    = "error"
	AccountStatusDisabled = "disabled"
)

// Account 为邮箱账号实体（表 mail_accounts）。
type Account struct {
	ID          uint64     `gorm:"column:id;primaryKey" json:"id"`
	UserID      string     `gorm:"column:user_id" json:"userId"`
	Provider    string     `gorm:"column:provider" json:"provider"`
	Email       string     `gorm:"column:email" json:"email"`
	DisplayName string     `gorm:"column:display_name" json:"displayName"`
	AuthType    string     `gorm:"column:auth_type" json:"authType"`
	Credential  string     `gorm:"column:credential" json:"-"` // 密文绝不外泄
	IMAPHost    string     `gorm:"column:imap_host" json:"imapHost"`
	IMAPPort    int        `gorm:"column:imap_port" json:"imapPort"`
	SMTPHost    string     `gorm:"column:smtp_host" json:"smtpHost"`
	SMTPPort    int        `gorm:"column:smtp_port" json:"smtpPort"`
	Status      string     `gorm:"column:status" json:"status"`
	SyncEnabled bool       `gorm:"column:sync_enabled" json:"syncEnabled"`
	UnreadCount int        `gorm:"column:unread_count" json:"unreadCount"`
	LastSyncAt  *time.Time `gorm:"column:last_sync_at" json:"lastSyncAt"`
	LastError   string     `gorm:"column:last_error" json:"lastError"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Account) TableName() string { return "mail_accounts" }

// Message 为邮件实体（表 mail_messages）。
type Message struct {
	ID            uint64     `gorm:"column:id;primaryKey" json:"id"`
	AccountID     uint64     `gorm:"column:account_id" json:"accountId"`
	UserID        string     `gorm:"column:user_id" json:"userId"`
	Folder        string     `gorm:"column:folder" json:"folder"`
	UID           uint32     `gorm:"column:uid" json:"uid"`
	MessageID     string     `gorm:"column:message_id" json:"messageId"`
	Subject       string     `gorm:"column:subject" json:"subject"`
	FromAddress   string     `gorm:"column:from_address" json:"fromAddress"`
	FromName      string     `gorm:"column:from_name" json:"fromName"`
	ToAddresses   string     `gorm:"column:to_addresses" json:"toAddresses"`
	ReceivedAt    *time.Time `gorm:"column:received_at" json:"receivedAt"`
	IsRead        bool       `gorm:"column:is_read" json:"isRead"`
	IsStarred     bool       `gorm:"column:is_starred" json:"isStarred"`
	HasAttachment bool       `gorm:"column:has_attachment" json:"hasAttachment"`
	Snippet       string     `gorm:"column:snippet" json:"snippet"`
	BodyText      string     `gorm:"column:body_text" json:"bodyText,omitempty"`
	BodyHTML      string     `gorm:"column:body_html" json:"bodyHtml,omitempty"`
	Size          int        `gorm:"column:size" json:"size"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Message) TableName() string { return "mail_messages" }

// SyncState 为文件夹同步游标（表 mail_sync_state）。
type SyncState struct {
	AccountID   uint64     `gorm:"column:account_id;primaryKey" json:"accountId"`
	Folder      string     `gorm:"column:folder;primaryKey" json:"folder"`
	UIDValidity uint32     `gorm:"column:uid_validity" json:"uidValidity"`
	LastUID     uint32     `gorm:"column:last_uid" json:"lastUid"`
	LastSyncAt  *time.Time `gorm:"column:last_sync_at" json:"lastSyncAt"`
	SyncStatus  string     `gorm:"column:sync_status" json:"syncStatus"`
	Error       string     `gorm:"column:error" json:"error"`
}

// TableName 指定表名。
func (SyncState) TableName() string { return "mail_sync_state" }
