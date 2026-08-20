// Package turnstile 提供 Cloudflare Turnstile 人机验证（登录页防 bot 撞库/暴力破解）。
//
// 集成方式：
//   - 前端登录页加载 Turnstile widget（https://challenges.cloudflare.com/turnstile/v0/api.js），
//     用户通过验证后获得一次性 token；
//   - 登录请求携带该 token，服务端调用 siteverify 校验（secret 仅存服务端，绝不下发前端）。
//
// 配置（缺省不启用，登录仍由限流 + 账户锁定兜底）：
//   - TURNSTILE_SITE_KEY   公开 site key（前端 widget 渲染用）
//   - TURNSTILE_SECRET_KEY 机密 secret key（仅服务端）
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// VerifyEndpoint 为 Turnstile siteverify 官方端点。
const VerifyEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Verifier 校验 Turnstile widget token。
type Verifier struct {
	secretKey string
	endpoint  string // 可覆盖（测试用）
	client    *http.Client
}

// NewVerifier 创建验证器。secretKey 为空时 Verify 恒返回 false（未配置）。
func NewVerifier(secretKey string) *Verifier {
	return &Verifier{
		secretKey: secretKey,
		endpoint:  VerifyEndpoint,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled 返回是否已配置 secret key（未配置则登录不强制验证）。
func (v *Verifier) Enabled() bool { return v != nil && v.secretKey != "" }

// siteVerifyResponse 为 siteverify 响应结构（仅取所需字段）。
type siteVerifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify 校验 token。remoteIP 传客户端真实 IP（可选，用于风控）。
// 返回 (通过?, 错误)。token 为空、网络失败或响应异常均返回错误（不通过）。
// 说明：校验失败按「拒绝」处理（fail-closed），与限流组件的 fail-open 策略不同——
// 人机验证为安全门禁，宁可误伤也不放行 bot。
func (v *Verifier) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if !v.Enabled() {
		return false, fmt.Errorf("turnstile 未配置")
	}
	if strings.TrimSpace(token) == "" {
		return false, fmt.Errorf("缺少人机验证 token")
	}
	form := url.Values{
		"secret":   {v.secretKey},
		"response": {token},
	}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("turnstile 验证请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return false, fmt.Errorf("turnstile 响应读取失败: %w", err)
	}
	var out siteVerifyResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("turnstile 响应解析失败: %w", err)
	}
	if !out.Success {
		// siteverify 明确拒绝：业务结果（验证不通过），非系统错误。
		return false, nil
	}
	return true, nil
}
