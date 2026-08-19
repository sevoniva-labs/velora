// 自助用户中心（Phase C4）：通过 Casdoor 管理 API 执行最小权限用户操作。
//
// 设计约束：
//   - 仅支持"当前登录用户更新自己的密码"这一种写操作；
//   - 调用方必须强校验目标 user id 与当前会话用户一致（handler 层完成）；
//   - 不提供任何全局管理端点（禁列表/禁删除/禁改角色），避免 admin token 权限放大。
package casdoor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// UpdateUserPassword 更新用户密码。
//
// Casdoor update-user 为全量更新，缺失必填字段（displayName 等）会被拒绝，
// 因此流程为：读取完整用户对象 → 仅替换 password → 提交。
// 这是最小权限写操作：只影响密码，不触碰其他用户属性。
func (c *Client) UpdateUserPassword(ctx context.Context, userID, newPassword string) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}

	// 1) 读取完整用户对象
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.issuer+"/api/get-user?id="+url.QueryEscape(userID), nil)
	if err != nil {
		return fmt.Errorf("构造读取用户请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Casdoor 读取用户失败: %w", err)
	}
	var payload struct {
		Status string         `json:"status"`
		Msg    string         `json:"msg"`
		Data   map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		resp.Body.Close()
		return fmt.Errorf("解析 Casdoor 用户响应失败: %w", err)
	}
	resp.Body.Close()
	if payload.Status != "ok" || payload.Data == nil {
		return fmt.Errorf("Casdoor 读取用户失败: %s", payload.Msg)
	}

	// 2) 仅替换 password 字段
	payload.Data["password"] = newPassword
	body, _ := json.Marshal(payload.Data)

	// 3) 提交全量对象，并显式声明只更新 password 列。
	//    Casdoor 默认更新列不含 password，必须通过 columns=password 指定。
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.issuer+"/api/update-user?id="+url.QueryEscape(userID)+"&columns=password", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造更新密码请求失败: %w", err)
	}
	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Accept", "application/json")
	resp2, err := c.http.Do(req2)
	if err != nil {
		return fmt.Errorf("请求 Casdoor 更新密码失败: %w", err)
	}
	defer resp2.Body.Close()
	var payload2 struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&payload2); err != nil {
		return fmt.Errorf("解析 Casdoor 更新密码响应失败: %w", err)
	}
	if payload2.Status != "ok" {
		return fmt.Errorf("Casdoor 更新密码失败: %s", payload2.Msg)
	}
	return nil
}

// VerifyPassword 用 ROPC 验证用户当前密码（改密前的旧密码校验）。
func (c *Client) VerifyPassword(ctx context.Context, username, password string) error {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("username", username)
	form.Set("password", password)
	form.Set("scope", "openid profile email")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.issuer+"/api/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("构造密码校验请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Casdoor 密码校验失败: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Msg         string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fmt.Errorf("解析 Casdoor 密码校验响应失败: %w", err)
	}
	if payload.AccessToken == "" {
		return fmt.Errorf("当前密码不正确")
	}
	return nil
}
