// Package casdoor 提供与 Casdoor 管理 API 的集成（只读同步，Velora 不修改 Casdoor 任何数据）。
package casdoor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SyncedApplication 为从 Casdoor 同步到的应用（OIDC 客户端）摘要。
type SyncedApplication struct {
	ClientID       string
	Name           string // Casdoor 唯一标识（小写英文），映射为门户应用编码
	DisplayName    string // 展示名，映射为门户应用名称
	Logo           string // 图标 URL（Casdoor 应用配置的 logo）
	HomepageURL    string // 主页地址
	Description    string
	GrantTypes     []string
	EnablePassword bool
}

// Client 通过 Casdoor 管理员凭据读取应用列表。
// 凭据仅用于服务端同步时获取管理 token，Velora 不存储 Casdoor 密码。
type Client struct {
	issuer       string
	clientID     string
	clientSecret string
	username     string
	password     string
	http         *http.Client
}

// NewClient 创建 Casdoor 同步客户端。
// clientID/clientSecret 为 Velora 在 Casdoor 注册的 OIDC 客户端凭据；
// adminUsername/adminPassword 为 Casdoor 管理员账号（用于读取全部应用列表）。
func NewClient(issuer, clientID, clientSecret, adminUsername, adminPassword string) *Client {
	return &Client{
		issuer:       strings.TrimSuffix(issuer, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
		username:     adminUsername,
		password:     adminPassword,
		http:         http.DefaultClient,
	}
}

// accessToken 通过 ROPC 获取管理员 token（与门户登录同一协议，仅服务端使用）。
func (c *Client) accessToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("username", c.username)
	form.Set("password", c.password)
	form.Set("scope", "openid profile email")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.issuer+"/api/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("构造 Casdoor 同步登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 Casdoor 同步登录失败: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Msg         string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("解析 Casdoor 同步登录响应失败: %w", err)
	}
	if payload.AccessToken == "" {
		msg := payload.Msg
		if msg == "" {
			msg = payload.Error
		}
		return "", fmt.Errorf("Casdoor 同步登录失败（请检查 CASDOOR_ADMIN_USERNAME/PASSWORD）: %s", msg)
	}
	return payload.AccessToken, nil
}

// FetchApplications 拉取 Casdoor 全部应用列表（OIDC 客户端）。
func (c *Client) FetchApplications(ctx context.Context) ([]SyncedApplication, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.issuer+"/api/get-applications?pageSize=100", nil)
	if err != nil {
		return nil, fmt.Errorf("构造应用列表请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Casdoor 应用列表失败: %w", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Status string              `json:"status"`
		Msg    string              `json:"msg"`
		Data   []casdoorAppPayload `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析 Casdoor 应用列表失败: %w", err)
	}
	if payload.Status != "ok" {
		return nil, fmt.Errorf("Casdoor 返回异常: %s", payload.Msg)
	}

	apps := make([]SyncedApplication, 0, len(payload.Data))
	for _, a := range payload.Data {
		if a.ClientID == "" {
			continue // 无 client id 的客户端无同步意义
		}
		apps = append(apps, SyncedApplication{
			ClientID:       a.ClientID,
			Name:           a.Name,
			DisplayName:    a.DisplayName,
			Logo:           a.Logo,
			HomepageURL:    a.HomepageURL,
			Description:    a.Description,
			GrantTypes:     a.GrantTypes,
			EnablePassword: a.EnablePassword,
		})
	}
	return apps, nil
}

type casdoorAppPayload struct {
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Logo           string   `json:"logo"`
	HomepageURL    string   `json:"homepageUrl"`
	Description    string   `json:"description"`
	ClientID       string   `json:"clientId"`
	GrantTypes     []string `json:"grantTypes"`
	EnablePassword bool     `json:"enablePassword"`
}
