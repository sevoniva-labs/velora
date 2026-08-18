package mail

import "context"

// Capabilities 描述 Provider 能力集：不同厂商能力不同，前端据此隐藏不可用操作。
type Capabilities struct {
	Idle     bool `json:"idle"`     // IMAP IDLE 实时推送（Phase 2）
	Send     bool `json:"send"`     // SMTP 发送（Phase 3）
	Reply    bool `json:"reply"`    // 回复/转发（Phase 3）
	Folders  bool `json:"folders"`  // 多文件夹
	Star     bool `json:"star"`     // 星标
	MarkRead bool `json:"markRead"` // 已读/未读
}

// Profile 为厂商预设：仅提供默认主机/端口配置，不改变任何业务逻辑。
// provider 可以是 aliyun，但上层代码完全不知道什么是阿里云。
type Profile struct {
	Provider string `json:"provider"`
	Label    string `json:"label"`
	IMAPHost string `json:"imapHost"`
	IMAPPort int    `json:"imapPort"`
	SMTPHost string `json:"smtpHost"`
	SMTPPort int    `json:"smtpPort"`
}

// BuiltinProfiles 内置厂商预设（绑定表单下拉选项；custom 为自定义 IMAP）。
var BuiltinProfiles = []Profile{
	{Provider: ProviderAliyun, Label: "阿里企业邮箱", IMAPHost: "imap.qiye.aliyun.com", IMAPPort: 993, SMTPHost: "smtp.qiye.aliyun.com", SMTPPort: 465},
	{Provider: ProviderTencent, Label: "腾讯企业邮箱", IMAPHost: "imap.exmail.qq.com", IMAPPort: 993, SMTPHost: "smtp.exmail.qq.com", SMTPPort: 465},
	{Provider: ProviderCustom, Label: "自定义 IMAP", IMAPHost: "", IMAPPort: 993, SMTPHost: "", SMTPPort: 465},
}

// ProfileOf 返回厂商预设；未知 provider 回退 custom。
func ProfileOf(provider string) Profile {
	for _, p := range BuiltinProfiles {
		if p.Provider == provider {
			return p
		}
	}
	return BuiltinProfiles[len(BuiltinProfiles)-1]
}

// FetchResult 为一次收件箱同步的结果。
type FetchResult struct {
	Messages    []*Message // 元数据 + 标记 + 摘要（正文按需另取）
	UIDValidity uint32
	LastUID     uint32
	UnseenTotal uint32 // 服务器端收件箱未读总数
}

// Provider 邮件供应商抽象：上层业务完全不感知具体邮箱厂商。
// 后续 Microsoft Graph / Exchange API 等以新实现接入，业务层零改动。
type Provider interface {
	// TestConnection 验证连接与登录可用。
	TestConnection(ctx context.Context, acc *Account, password string) error
	// FetchInbox 拉取收件箱最近 limit 封邮件的元数据与标记。
	FetchInbox(ctx context.Context, acc *Account, password string, limit int) (*FetchResult, error)
	// FetchBody 按需拉取正文（text / html 各一份，可为空）。
	FetchBody(ctx context.Context, acc *Account, password string, folder string, uid uint32) (text string, html string, err error)
	// SetFlags 同步已读 / 星标标记到服务器（nil 表示不修改该项）。
	SetFlags(ctx context.Context, acc *Account, password string, folder string, uid uint32, seen *bool, starred *bool) error
	// Capabilities 返回能力集。
	Capabilities() Capabilities
}

// NewProvider 按账号 provider 构造实现。当前统一走 Generic IMAP，
// 厂商差异收敛在 Profile 默认值；未来 Graph/Exchange 在此分流。
func NewProvider(provider string) Provider {
	return NewIMAPProvider()
}
