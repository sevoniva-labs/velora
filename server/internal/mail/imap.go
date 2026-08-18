package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"strings"
	"time"

	imap "github.com/emersion/go-imap"
	imapclient "github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// IMAPProvider 通用 IMAP 实现：阿里/腾讯/自建企业邮箱统一走这里，
// 厂商差异只体现在 Profile 默认主机配置（见 provider.go）。
type IMAPProvider struct {
	DialTimeout time.Duration
	OpTimeout   time.Duration
}

// NewIMAPProvider 创建通用 IMAP Provider。
func NewIMAPProvider() *IMAPProvider {
	return &IMAPProvider{DialTimeout: 10 * time.Second, OpTimeout: 60 * time.Second}
}

// Capabilities 当前能力：已读/星标标记同步；IDLE 与 SMTP 属后续阶段。
func (p *IMAPProvider) Capabilities() Capabilities {
	return Capabilities{Idle: false, Send: false, Reply: false, Folders: false, Star: true, MarkRead: true}
}

// dial 建立 TLS 连接并完成登录；调用方负责 Logout。
func (p *IMAPProvider) dial(acc *Account, password string) (*imapclient.Client, error) {
	addr := net.JoinHostPort(acc.IMAPHost, fmt.Sprintf("%d", acc.IMAPPort))
	dialer := &net.Dialer{Timeout: p.DialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: acc.IMAPHost}) //nolint:gosec // ServerName 已固定为服务器主机
	if err != nil {
		return nil, fmt.Errorf("无法连接邮箱服务器 %s，请检查服务器地址与网络", addr)
	}
	c, err := imapclient.New(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("IMAP 握手失败：%s", addr)
	}
	if p.OpTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(p.OpTimeout))
	}
	if err := c.Login(acc.Email, password); err != nil {
		_ = c.Logout()
		return nil, fmt.Errorf("邮箱登录失败：请确认邮箱地址与授权码（非登录密码），并确认已开启 IMAP 服务")
	}
	return c, nil
}

// TestConnection 验证连接与登录可用。
func (p *IMAPProvider) TestConnection(ctx context.Context, acc *Account, password string) error {
	c, err := p.dial(acc, password)
	if err != nil {
		return err
	}
	defer func() { _ = c.Logout() }()
	if _, err := c.Select("INBOX", true); err != nil {
		return fmt.Errorf("无法打开收件箱")
	}
	return nil
}

// FetchInbox 拉取收件箱最近 limit 封邮件的元数据与标记（不含正文，正文按需 FetchBody）。
func (p *IMAPProvider) FetchInbox(ctx context.Context, acc *Account, password string, limit int) (*FetchResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	c, err := p.dial(acc, password)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Logout() }()

	mbox, err := c.Select("INBOX", true)
	if err != nil {
		return nil, fmt.Errorf("无法打开收件箱")
	}
	res := &FetchResult{UIDValidity: mbox.UidValidity, UnseenTotal: mbox.Unseen}
	if mbox.Messages == 0 {
		return res, nil
	}

	uids, err := c.UidSearch(imap.NewSearchCriteria())
	if err != nil {
		return nil, fmt.Errorf("检索邮件列表失败")
	}
	if len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}
	if len(uids) == 0 {
		return res, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uids...)
	items := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, imap.FetchBodyStructure, imap.FetchRFC822Size}

	ch := make(chan *imap.Message, 16)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(seqset, items, ch) }()

	var maxUID uint32
	for m := range ch {
		res.Messages = append(res.Messages, convertEnvelope(acc, m))
		if m.Uid > maxUID {
			maxUID = m.Uid
		}
	}
	if err := <-done; err != nil {
		return nil, fmt.Errorf("拉取邮件元数据失败")
	}
	res.LastUID = maxUID
	if mbox.UidNext > 0 && mbox.UidNext-1 > res.LastUID {
		res.LastUID = mbox.UidNext - 1
	}
	return res, nil
}

// FetchBody 按需拉取正文（PEEK 模式，不触发服务端已读）。
func (p *IMAPProvider) FetchBody(ctx context.Context, acc *Account, password string, folder string, uid uint32) (string, string, error) {
	c, err := p.dial(acc, password)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = c.Logout() }()
	if _, err := c.Select(folder, true); err != nil {
		return "", "", fmt.Errorf("无法打开邮箱文件夹")
	}

	section := &imap.BodySectionName{Peek: true}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	ch := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(seqset, []imap.FetchItem{section.FetchItem()}, ch) }()
	m, ok := <-ch
	if !ok || m == nil {
		return "", "", fmt.Errorf("邮件不存在或已在服务器端被移动")
	}
	if err := <-done; err != nil {
		return "", "", fmt.Errorf("正文拉取失败")
	}
	// 响应中的 section 键由服务端回包解析重建，必须用 GetBody 归一化匹配。
	r := m.GetBody(section)
	if r == nil {
		return "", "", fmt.Errorf("邮件正文为空")
	}
	return parseBodies(r)
}

// SetFlags 同步已读 / 星标标记到服务器（SILENT 模式；nil 表示不修改该项）。
func (p *IMAPProvider) SetFlags(ctx context.Context, acc *Account, password string, folder string, uid uint32, seen *bool, starred *bool) error {
	c, err := p.dial(acc, password)
	if err != nil {
		return err
	}
	defer func() { _ = c.Logout() }()
	if _, err := c.Select(folder, false); err != nil {
		return fmt.Errorf("无法打开邮箱文件夹")
	}
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	apply := func(flag string, on bool) error {
		op := imap.FlagsOp(imap.AddFlags)
		if !on {
			op = imap.FlagsOp(imap.RemoveFlags)
		}
		return c.UidStore(seqset, imap.FormatFlagsOp(op, true), []interface{}{flag}, nil)
	}
	if seen != nil {
		if err := apply(imap.SeenFlag, *seen); err != nil {
			return fmt.Errorf("同步已读状态失败")
		}
	}
	if starred != nil {
		if err := apply(imap.FlaggedFlag, *starred); err != nil {
			return fmt.Errorf("同步星标失败")
		}
	}
	return nil
}

// convertEnvelope 将 IMAP 信封转换为邮件实体（元数据 + 标记）。
func convertEnvelope(acc *Account, m *imap.Message) *Message {
	msg := &Message{
		AccountID: acc.ID,
		UserID:    acc.UserID,
		Folder:    "INBOX",
		UID:       m.Uid,
		Size:      int(m.Size),
	}
	if !m.InternalDate.IsZero() {
		t := m.InternalDate
		msg.ReceivedAt = &t
	}
	for _, f := range m.Flags {
		switch f {
		case imap.SeenFlag:
			msg.IsRead = true
		case imap.FlaggedFlag:
			msg.IsStarred = true
		}
	}
	if env := m.Envelope; env != nil {
		dec := new(mime.WordDecoder)
		if s, err := dec.DecodeHeader(env.Subject); err == nil {
			msg.Subject = truncateRunes(s, 512)
		} else {
			msg.Subject = truncateRunes(env.Subject, 512)
		}
		msg.MessageID = env.MessageId
		if len(env.From) > 0 {
			a := env.From[0]
			msg.FromAddress = a.MailboxName + "@" + a.HostName
			if n, err := dec.DecodeHeader(a.PersonalName); err == nil {
				msg.FromName = n
			} else {
				msg.FromName = a.PersonalName
			}
		}
		var tos []string
		for _, a := range env.To {
			tos = append(tos, a.MailboxName+"@"+a.HostName)
		}
		msg.ToAddresses = truncateRunes(strings.Join(tos, ", "), 512)
		if !env.Date.IsZero() && msg.ReceivedAt == nil {
			t := env.Date
			msg.ReceivedAt = &t
		}
	}
	msg.HasAttachment = hasAttachment(m.BodyStructure)
	return msg
}

// hasAttachment 递归判断 BODYSTRUCTURE 是否含附件（仅元数据，不下载附件本体）。
func hasAttachment(bs *imap.BodyStructure) bool {
	if bs == nil {
		return false
	}
	if strings.EqualFold(bs.Disposition, "attachment") {
		return true
	}
	if bs.DispositionParams["filename"] != "" || bs.Params["name"] != "" {
		if !strings.EqualFold(bs.MIMEType, "multipart") {
			return true
		}
	}
	for _, part := range bs.Parts {
		if hasAttachment(part) {
			return true
		}
	}
	return false
}

// parseBodies 解析 RFC822 正文：提取 text/plain 与 text/html（附件正文不下载）。
func parseBodies(r io.Reader) (string, string, error) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		// 非 MIME 邮件：整体按纯文本处理。
		b, _ := io.ReadAll(io.LimitReader(r, 2<<20))
		return string(b), "", nil
	}
	var text, html string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, _ := io.ReadAll(io.LimitReader(part.Body, 2<<20))
			switch strings.ToLower(ct) {
			case "text/plain":
				if text == "" {
					text = string(b)
				}
			case "text/html":
				if html == "" {
					html = string(b)
				}
			}
		case *mail.AttachmentHeader:
			// 附件仅保留 has_attachment 元数据标记，正文不下载（Phase 3 按需拉取）。
		}
	}
	return text, html, nil
}

func truncateRunes(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) > n {
		return string(rs[:n])
	}
	return string(rs)
}
