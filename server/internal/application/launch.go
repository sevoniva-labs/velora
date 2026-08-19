package application

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// LaunchResult 为应用启动结果。
type LaunchResult struct {
	Type   string `json:"type"`   // redirect（同页跳转）| url（新窗口打开）
	URL    string `json:"url"`    // 服务端根据受信配置生成的启动地址
	Target string `json:"target"` // _self | _blank
}

// LaunchProvider 为应用启动扩展点。
//
// 第一阶段实现：URL / OIDC；后续可扩展 SAML / CAS / ForwardAuth，
// 避免在业务代码中堆叠 if ssoType == ... 分支。
type LaunchProvider interface {
	Type() string
	Launch(ctx context.Context, app *Application, user *auth.CurrentUser) (*LaunchResult, error)
}

// URLLaunchProvider 直接返回受信配置的启动地址（URL 类应用）。
type URLLaunchProvider struct{}

func (URLLaunchProvider) Type() string { return SSOTypeURL }

func (URLLaunchProvider) Launch(_ context.Context, app *Application, _ *auth.CurrentUser) (*LaunchResult, error) {
	target := strings.TrimSpace(app.LaunchURL)
	if target == "" {
		target = strings.TrimSpace(app.HomeURL)
	}
	if target == "" {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "应用未配置有效的启动地址")
	}
	if err := validateURLField(target, "应用启动地址"); err != nil {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "应用未配置有效的启动地址")
	}
	return &LaunchResult{Type: "url", URL: target, Target: "_blank"}, nil
}

// OIDCLaunchProvider 生成 Casdoor 为该应用签发的登录跳转（OIDC 类应用）。
// 用户点击后进入 Casdoor，使用该应用的 client_id 完成认证并回到应用自身的 redirect_uri。
type OIDCLaunchProvider struct {
	issuer string
}

func (OIDCLaunchProvider) Type() string { return SSOTypeOIDC }

func (p OIDCLaunchProvider) Launch(ctx context.Context, app *Application, user *auth.CurrentUser) (*LaunchResult, error) {
	if strings.TrimSpace(app.CasdoorClientID) == "" {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该 OIDC 应用未配置 Casdoor Client ID")
	}
	redirectURI := strings.TrimSpace(app.LaunchURL)
	if redirectURI == "" {
		redirectURI = strings.TrimSpace(app.HomeURL)
	}
	if err := validateURLField(redirectURI, "OIDC 回调地址"); err != nil {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该 OIDC 应用未配置有效的回调地址")
	}

	issuer := strings.TrimRight(p.issuer, "/")
	authorizeURL := fmt.Sprintf("%s/login/oauth/authorize", issuer)
	state, err := auth.RandomToken(16)
	if err != nil {
		return nil, errs.Internal("生成 OIDC state 失败", err)
	}

	u := url.Values{}
	u.Set("client_id", app.CasdoorClientID)
	u.Set("redirect_uri", redirectURI)
	u.Set("response_type", "code")
	u.Set("scope", "openid profile email")
	u.Set("state", state)
	// 携带当前 Velora 用户上下文，方便目标应用展示身份（可选字段）。
	if user != nil && user.Username != "" {
		u.Set("login_hint", user.Username)
	}

	return &LaunchResult{Type: "url", URL: authorizeURL + "?" + u.Encode(), Target: "_blank"}, nil
}

// VeloraOIDCLaunchProvider 生成 Velora 自身 OIDC Provider 的登录跳转（VELORA_OIDC 类应用）。
// 这是"统一登录入口"目标的核心路径：应用 SSO 登录 → Velora /oidc/authorize
// （未登录先落 Velora 登录页），Casdoor 对应用完全不可见。
type VeloraOIDCLaunchProvider struct {
	// publicBaseURL Velora 对外地址（issuer）。
	publicBaseURL string
	// clientResolver 按应用 ID 查询 Velora OIDC client 的回调（由组装层注入，
	// 避免 application 包反向依赖 oidcprovider）。
	clientResolver func(ctx context.Context, applicationID uint64) (clientID string, redirectURIs []string, ok bool, err error)
}

func (VeloraOIDCLaunchProvider) Type() string { return SSOTypeVeloraOIDC }

func (p VeloraOIDCLaunchProvider) Launch(ctx context.Context, app *Application, user *auth.CurrentUser) (*LaunchResult, error) {
	if p.clientResolver == nil {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该应用未配置 Velora OIDC 客户端")
	}
	clientID, redirectURIs, ok, err := p.clientResolver(ctx, app.ID)
	if err != nil {
		return nil, errs.Internal("读取 OIDC 客户端失败", err)
	}
	if !ok || clientID == "" {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该应用未配置 Velora OIDC 客户端，请先在管理后台生成")
	}
	if len(redirectURIs) == 0 {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该应用未配置 Velora OIDC 回调地址白名单")
	}

	issuer := strings.TrimRight(p.publicBaseURL, "/")
	authorizeURL := issuer + "/oidc/authorize"
	state, err := auth.RandomToken(16)
	if err != nil {
		return nil, errs.Internal("生成 OIDC state 失败", err)
	}

	u := url.Values{}
	u.Set("client_id", clientID)
	u.Set("redirect_uri", redirectURIs[0])
	u.Set("response_type", "code")
	u.Set("scope", "openid profile email")
	u.Set("state", state)
	u.Set("code_challenge_method", "S256")
	challenge, _ := auth.RandomToken(24)
	u.Set("code_challenge", challenge) // 简化：一次性随机串作为 challenge（对应应用侧需同值 verifier）
	if user != nil && user.Username != "" {
		u.Set("login_hint", user.Username)
	}

	return &LaunchResult{Type: "url", URL: authorizeURL + "?" + u.Encode(), Target: "_blank"}, nil
}

// LaunchRegistry 按 SSO 类型分发 Provider。
type LaunchRegistry struct {
	providers map[string]LaunchProvider
}

// NewLaunchRegistry 创建 Provider 注册表。
// publicBaseURL 为 Velora 对外地址（VELORA_OIDC issuer）；
// clientResolver 按应用 ID 解析 Velora OIDC client（nil 时 VELORA_OIDC 返回不支持）。
func NewLaunchRegistry(oidcIssuer, publicBaseURL string, clientResolver func(ctx context.Context, applicationID uint64) (clientID string, redirectURIs []string, ok bool, err error)) *LaunchRegistry {
	return &LaunchRegistry{
		providers: map[string]LaunchProvider{
			SSOTypeURL:         URLLaunchProvider{},
			SSOTypeOIDC:        OIDCLaunchProvider{issuer: oidcIssuer},
			SSOTypeVeloraOIDC:  VeloraOIDCLaunchProvider{publicBaseURL: publicBaseURL, clientResolver: clientResolver},
			SSOTypeForwardAuth: ForwardAuthLaunchProvider{publicBaseURL: publicBaseURL},
		},
	}
}

// Provider 返回对应类型的 Provider；未实现类型返回 nil。
func (r *LaunchRegistry) Provider(ssoType string) LaunchProvider {
	return r.providers[ssoType]
}

// Launch 执行应用启动（调用方需已完成权限与状态校验）。
func (r *LaunchRegistry) Launch(ctx context.Context, app *Application, user *auth.CurrentUser) (*LaunchResult, error) {
	p := r.Provider(app.SSOType)
	if p == nil {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该应用接入类型（"+app.SSOType+"）暂未支持，请联系管理员")
	}
	return p.Launch(ctx, app, user)
}
