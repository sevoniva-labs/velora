package mail

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/todo"
)

// inboxFolder 第一阶段只同步收件箱。
const inboxFolder = "INBOX"

// syncLimit 单次同步拉取的最近邮件数量。
const syncLimit = 50

// Service 邮件业务服务：账号 / 同步 / 消息 / 转待办。
type Service struct {
	db          *gorm.DB
	cipher      *CredentialCipher
	todos       *todo.Service
	newProvider func(provider string) Provider
}

// NewService 创建邮件服务。
func NewService(db *gorm.DB, cipher *CredentialCipher, todos *todo.Service) *Service {
	return &Service{db: db, cipher: cipher, todos: todos, newProvider: NewProvider}
}

// ---------- 账号 ----------

// CreateAccountInput 为绑定邮箱的入参（Password 为授权码，仅用于加密存储，永不落明文）。
type CreateAccountInput struct {
	Provider    string
	Email       string
	Password    string
	DisplayName string
	IMAPHost    string
	IMAPPort    int
	SMTPHost    string
	SMTPPort    int
}

// ListAccounts 我的邮箱账号列表。
func (s *Service) ListAccounts(ctx context.Context, userID string) ([]Account, error) {
	var accounts []Account
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id ASC").
		Find(&accounts).Error; err != nil {
		return nil, errs.DB(err)
	}
	return accounts, nil
}

// CreateAccount 绑定邮箱：先实测连接，通过后才落库（凭证 AES-256-GCM 加密）。
func (s *Service) CreateAccount(ctx context.Context, userID string, in CreateAccountInput) (*Account, error) {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.Provider = strings.TrimSpace(in.Provider)
	if in.Provider == "" {
		in.Provider = ProviderAliyun
	}
	if !strings.Contains(in.Email, "@") {
		return nil, errs.InvalidParam("邮箱地址格式不正确")
	}
	if strings.TrimSpace(in.Password) == "" {
		return nil, errs.InvalidParam("请填写邮箱授权码（非登录密码）")
	}
	profile := ProfileOf(in.Provider)
	if strings.TrimSpace(in.IMAPHost) == "" {
		in.IMAPHost = profile.IMAPHost
	}
	if in.IMAPPort <= 0 {
		in.IMAPPort = profile.IMAPPort
	}
	if strings.TrimSpace(in.SMTPHost) == "" {
		in.SMTPHost = profile.SMTPHost
	}
	if in.SMTPPort <= 0 {
		in.SMTPPort = profile.SMTPPort
	}
	if in.IMAPHost == "" {
		return nil, errs.InvalidParam("自定义 IMAP 需填写服务器地址")
	}

	acc := &Account{
		UserID:      userID,
		Provider:    in.Provider,
		Email:       in.Email,
		DisplayName: strings.TrimSpace(in.DisplayName),
		AuthType:    "password",
		IMAPHost:    strings.TrimSpace(in.IMAPHost),
		IMAPPort:    in.IMAPPort,
		SMTPHost:    strings.TrimSpace(in.SMTPHost),
		SMTPPort:    in.SMTPPort,
		Status:      AccountStatusActive,
		SyncEnabled: true,
	}

	// 先验证连接，失败则不入库（避免用户反复解绑重试）。
	if err := s.newProvider(acc.Provider).TestConnection(ctx, acc, in.Password); err != nil {
		return nil, errs.New(errs.CodeMailSyncFailed, 502, err.Error())
	}

	cred, err := s.cipher.Encrypt(in.Password)
	if err != nil {
		return nil, errs.Internal("凭证加密失败", err)
	}
	acc.Credential = cred
	now := time.Now()
	acc.CreatedAt = now
	acc.UpdatedAt = now
	if err := s.db.WithContext(ctx).Create(acc).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, errs.New(errs.CodeMailAccountExists, 409, "该邮箱已绑定，请勿重复添加")
		}
		return nil, errs.DB(err)
	}
	return acc, nil
}

// DeleteAccount 解绑邮箱（邮件数据随外键级联删除）。
func (s *Service) DeleteAccount(ctx context.Context, userID string, id uint64) error {
	res := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&Account{})
	if res.Error != nil {
		return errs.DB(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.NotFound(errs.CodeMailAccountNotFound, "邮箱账号不存在")
	}
	return nil
}

// TestAccount 测试已绑定账号的连接（用于排障）。
func (s *Service) TestAccount(ctx context.Context, userID string, id uint64) error {
	acc, err := s.getOwnedAccount(ctx, userID, id)
	if err != nil {
		return err
	}
	password, err := s.cipher.Decrypt(acc.Credential)
	if err != nil {
		return errs.Internal("凭证解密失败", err)
	}
	if err := s.newProvider(acc.Provider).TestConnection(ctx, acc, password); err != nil {
		s.recordAccountError(ctx, acc.ID, err)
		return err
	}
	s.db.WithContext(ctx).Model(&Account{}).Where("id = ?", acc.ID).
		Updates(map[string]any{"status": AccountStatusActive, "last_error": "", "updated_at": time.Now()})
	return nil
}

// ---------- 同步 ----------

// SyncAccount 同步单个账号收件箱（手动触发或定时补偿）。
func (s *Service) SyncAccount(ctx context.Context, userID string, id uint64) (*Account, error) {
	acc, err := s.getOwnedAccount(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if err := s.syncOne(ctx, acc); err != nil {
		return nil, err
	}
	_ = s.db.WithContext(ctx).First(acc, acc.ID).Error
	return acc, nil
}

// SyncAll 定时补偿同步：遍历全部启用了同步的账号。返回成功数。
func (s *Service) SyncAll(ctx context.Context) (int, error) {
	var accounts []Account
	if err := s.db.WithContext(ctx).
		Where("sync_enabled = ? AND status <> ?", true, AccountStatusDisabled).
		Find(&accounts).Error; err != nil {
		return 0, errs.DB(err)
	}
	ok := 0
	for i := range accounts {
		accCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		if err := s.syncOne(accCtx, &accounts[i]); err == nil {
			ok++
		}
		cancel()
	}
	return ok, nil
}

// syncOne 执行一次收件箱同步：拉元数据 → upsert → 更新游标与未读数。
func (s *Service) syncOne(ctx context.Context, acc *Account) error {
	password, err := s.cipher.Decrypt(acc.Credential)
	if err != nil {
		return errs.Internal("凭证解密失败", err)
	}
	res, err := s.newProvider(acc.Provider).FetchInbox(ctx, acc, password, syncLimit)
	if err != nil {
		s.recordAccountError(ctx, acc.ID, err)
		return errs.New(errs.CodeMailSyncFailed, 502, err.Error())
	}

	now := time.Now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, m := range res.Messages {
			// 幂等：按（账号, 文件夹, UID）upsert；已缓存的正文不覆盖。
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "account_id"}, {Name: "folder"}, {Name: "uid"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"subject", "from_address", "from_name", "to_addresses", "message_id",
					"received_at", "is_read", "is_starred", "has_attachment", "size", "updated_at",
				}),
			}).Create(m).Error; err != nil {
				return err
			}
		}
		state := SyncState{
			AccountID:   acc.ID,
			Folder:      inboxFolder,
			UIDValidity: res.UIDValidity,
			LastUID:     res.LastUID,
			LastSyncAt:  &now,
			SyncStatus:  "ok",
			Error:       "",
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "account_id"}, {Name: "folder"}},
			DoUpdates: clause.AssignmentColumns([]string{"uid_validity", "last_uid", "last_sync_at", "sync_status", "error"}),
		}).Create(&state).Error; err != nil {
			return err
		}
		return tx.Model(&Account{}).Where("id = ?", acc.ID).Updates(map[string]any{
			"status":       AccountStatusActive,
			"last_sync_at": now,
			"last_error":   "",
			"unread_count": int(res.UnseenTotal),
			"updated_at":   now,
		}).Error
	})
	if err != nil {
		return errs.DB(err)
	}
	return nil
}

// recordAccountError 记录账号级错误（同步/连接失败）。
func (s *Service) recordAccountError(ctx context.Context, accountID uint64, cause error) {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	s.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).
		Updates(map[string]any{"status": AccountStatusError, "last_error": msg, "updated_at": time.Now()})
	s.db.WithContext(ctx).Model(&SyncState{}).
		Where("account_id = ? AND folder = ?", accountID, inboxFolder).
		Updates(map[string]any{"sync_status": "error", "error": msg})
}

// ---------- 消息 ----------

// MessageFilter 邮件列表过滤。
type MessageFilter struct {
	AccountID uint64
	Unread    *bool
	Starred   *bool
	Keyword   string
	Page      int
	PageSize  int
}

// ListMessages 邮件列表（不含正文，按收件时间倒序分页）。
func (s *Service) ListMessages(ctx context.Context, userID string, f MessageFilter) ([]Message, int64, error) {
	q := s.db.WithContext(ctx).Model(&Message{}).Where("user_id = ? AND folder = ?", userID, inboxFolder)
	if f.AccountID > 0 {
		q = q.Where("account_id = ?", f.AccountID)
	}
	if f.Unread != nil && *f.Unread {
		q = q.Where("is_read = ?", false)
	}
	if f.Starred != nil && *f.Starred {
		q = q.Where("is_starred = ?", true)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("subject ILIKE ? OR from_address ILIKE ? OR from_name ILIKE ?", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errs.DB(err)
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	var items []Message
	if err := q.Omit("body_text", "body_html").
		Order("received_at DESC NULLS LAST").Order("id DESC").
		Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, errs.DB(err)
	}
	return items, total, nil
}

// GetMessage 邮件详情：正文未缓存时按需从服务器拉取并落库；打开即置为已读。
// bodyErr 为非致命错误（正文拉取失败仍返回元数据）。
func (s *Service) GetMessage(ctx context.Context, userID string, id uint64) (*Message, string, error) {
	msg, err := s.getOwnedMessage(ctx, userID, id)
	if err != nil {
		return nil, "", err
	}
	bodyErr := ""
	if msg.BodyText == "" && msg.BodyHTML == "" {
		if text, html, ferr := s.fetchAndCacheBody(ctx, msg); ferr != nil {
			bodyErr = ferr.Error()
		} else {
			msg.BodyText, msg.BodyHTML = text, html
		}
	}
	if !msg.IsRead {
		_ = s.setRead(ctx, msg, true)
		msg.IsRead = true
	}
	return msg, bodyErr, nil
}

// fetchAndCacheBody 拉取正文并缓存（同时回填摘要）。
func (s *Service) fetchAndCacheBody(ctx context.Context, msg *Message) (string, string, error) {
	acc, err := s.getOwnedAccount(ctx, msg.UserID, msg.AccountID)
	if err != nil {
		return "", "", err
	}
	password, err := s.cipher.Decrypt(acc.Credential)
	if err != nil {
		return "", "", errs.Internal("凭证解密失败", err)
	}
	text, html, err := s.newProvider(acc.Provider).FetchBody(ctx, acc, password, msg.Folder, msg.UID)
	if err != nil {
		return "", "", err
	}
	snippet := msg.Snippet
	if plain := text; plain != "" {
		snippet = truncateRunes(plain, 160)
	} else if html != "" {
		snippet = truncateRunes(stripHTML(html), 160)
	}
	updates := map[string]any{"body_text": text, "body_html": html, "snippet": snippet, "updated_at": time.Now()}
	if err := s.db.WithContext(ctx).Model(&Message{}).Where("id = ?", msg.ID).Updates(updates).Error; err != nil {
		return "", "", errs.DB(err)
	}
	msg.Snippet = snippet
	return text, html, nil
}

// SetRead 设置已读/未读（同步服务器标记 + 本地状态 + 未读数）。
func (s *Service) SetRead(ctx context.Context, userID string, id uint64, read bool) error {
	msg, err := s.getOwnedMessage(ctx, userID, id)
	if err != nil {
		return err
	}
	if msg.IsRead == read {
		return nil
	}
	return s.setRead(ctx, msg, read)
}

func (s *Service) setRead(ctx context.Context, msg *Message, read bool) error {
	acc, err := s.getOwnedAccount(ctx, msg.UserID, msg.AccountID)
	if err != nil {
		return err
	}
	password, err := s.cipher.Decrypt(acc.Credential)
	if err != nil {
		return errs.Internal("凭证解密失败", err)
	}
	if err := s.newProvider(acc.Provider).SetFlags(ctx, acc, password, msg.Folder, msg.UID, &read, nil); err != nil {
		return errs.New(errs.CodeMailSyncFailed, 502, err.Error())
	}
	delta := -1
	if !read {
		delta = 1
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Message{}).Where("id = ?", msg.ID).
			Updates(map[string]any{"is_read": read, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		// 未读数在 [0, +∞) 区间调整，下次全量同步会校准为服务端真实值。
		return tx.Model(&Account{}).Where("id = ?", msg.AccountID).
			Update("unread_count", gorm.Expr("GREATEST(unread_count + ?, 0)", delta)).Error
	})
}

// SetStarred 设置星标（同步服务器标记 + 本地状态）。
func (s *Service) SetStarred(ctx context.Context, userID string, id uint64, starred bool) error {
	msg, err := s.getOwnedMessage(ctx, userID, id)
	if err != nil {
		return err
	}
	if msg.IsStarred == starred {
		return nil
	}
	acc, err := s.getOwnedAccount(ctx, msg.UserID, msg.AccountID)
	if err != nil {
		return err
	}
	password, err := s.cipher.Decrypt(acc.Credential)
	if err != nil {
		return errs.Internal("凭证解密失败", err)
	}
	if err := s.newProvider(acc.Provider).SetFlags(ctx, acc, password, msg.Folder, msg.UID, nil, &starred); err != nil {
		return errs.New(errs.CodeMailSyncFailed, 502, err.Error())
	}
	if err := s.db.WithContext(ctx).Model(&Message{}).Where("id = ?", msg.ID).
		Updates(map[string]any{"is_starred": starred, "updated_at": time.Now()}).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

// ---------- 转待办 ----------

// ConvertInput 为"转为待办"的入参。
type ConvertInput struct {
	Title    string
	Priority string
	Kind     string
	DueAt    *time.Time
}

// ConvertToTodo 把邮件转为待办（幂等：同一邮件重复转换只更新，不产生重复待办）。
// 关联方式：todos.source_system='mail' + source_id=mail_messages.id——复用既有
// 待办幂等机制，不改 Todo 主表结构、不新建桥接表。
func (s *Service) ConvertToTodo(ctx context.Context, userID string, id uint64, in ConvertInput) (*todo.Todo, error) {
	msg, err := s.getOwnedMessage(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = msg.Subject
	}
	if title == "" {
		title = "（无主题邮件）"
	}
	priority := in.Priority
	if priority == "" {
		priority = todo.PriorityMid
	}
	if !todo.ValidPriority(priority) {
		return nil, errs.InvalidParam("priority 仅支持 urgent/high/mid/low")
	}
	kind := in.Kind
	if kind == "" {
		kind = todo.KindOther
	}
	if !todo.ValidKind(kind) {
		return nil, errs.InvalidParam("kind 仅支持 mail/approval/devops/ops/project/hr/other")
	}
	t, err := s.todos.Upsert(ctx, todo.Todo{
		UserID:       userID,
		Title:        title,
		Kind:         kind,
		SourceSystem: "mail",
		SourceLabel:  "企业邮箱",
		SourceID:     strconv.FormatUint(msg.ID, 10),
		Priority:     priority,
		DueAt:        in.DueAt,
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ---------- 内部 ----------

func (s *Service) getOwnedAccount(ctx context.Context, userID string, id uint64) (*Account, error) {
	var acc Account
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&acc).Error; err != nil {
		return nil, errs.NotFound(errs.CodeMailAccountNotFound, "邮箱账号不存在")
	}
	return &acc, nil
}

func (s *Service) getOwnedMessage(ctx context.Context, userID string, id uint64) (*Message, error) {
	var msg Message
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&msg).Error; err != nil {
		return nil, errs.NotFound(errs.CodeMailMessageNotFound, "邮件不存在")
	}
	return &msg, nil
}

// isUniqueViolation 判定 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505")
}

var (
	htmlTagRe   = regexp.MustCompile(`<[^>]*>`)
	htmlStyleRe = regexp.MustCompile(`(?is)<(style|script)[^>]*>.*?</(style|script)>`)
)

// stripHTML 粗粒度去标签（仅用于摘要回填；前端渲染走 DOMPurify 消毒）。
// 先剔除 style/script 整块内容，避免 CSS 泄漏进摘要。
func stripHTML(s string) string {
	s = htmlStyleRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(htmlTagRe.ReplaceAllString(s, " ")), " ")
}
