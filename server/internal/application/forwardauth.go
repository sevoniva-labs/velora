package application

import (
	"context"
	"net/url"
	"strings"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// ForwardAuthLaunchProvider 为非 OIDC 老系统（Nginx/APISIX auth_request）套 SSO（Phase D1）。
//
// 启动结果跳转到 Velora 的 forward-auth 校验端点；老系统所在网关配置
// `auth_request /forward-auth`，Velora 校验会话：有效返回 200 并注入
// X-Velora-User 身份头，无效返回 401（网关转跳 Velora 登录页）。
type ForwardAuthLaunchProvider struct {
	// publicBaseURL Velora 对外地址。
	publicBaseURL string
}

func (ForwardAuthLaunchProvider) Type() string { return SSOTypeForwardAuth }

func (p ForwardAuthLaunchProvider) Launch(ctx context.Context, app *Application, user *auth.CurrentUser) (*LaunchResult, error) {
	target := strings.TrimSpace(app.LaunchURL)
	if target == "" {
		target = strings.TrimSpace(app.HomeURL)
	}
	if err := validateURLField(target, "应用地址"); err != nil {
		return nil, errs.New(errs.CodeApplicationDisabled, 400, "该应用未配置有效的目标地址")
	}
	base := strings.TrimRight(p.publicBaseURL, "/")
	u := url.Values{}
	u.Set("next", target)
	return &LaunchResult{
		Type:   "redirect",
		URL:    base + "/forward-auth?" + u.Encode(),
		Target: "_self",
	}, nil
}
