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

// LaunchRegistry 按 SSO 类型分发 Provider。
type LaunchRegistry struct {
	providers map[string]LaunchProvider
}

// NewLaunchRegistry 创建 Provider 注册表。
func NewLaunchRegistry(oidcIssuer string) *LaunchRegistry {
	return &LaunchRegistry{
		providers: map[string]LaunchProvider{
			SSOTypeURL:  URLLaunchProvider{},
			SSOTypeOIDC: OIDCLaunchProvider{issuer: oidcIssuer},
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
