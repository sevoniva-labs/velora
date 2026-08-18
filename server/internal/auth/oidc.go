package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// OIDCManager 封装与 Casdoor 的 OIDC Authorization Code + PKCE 交互。
// Provider Discovery 惰性初始化：服务启动不依赖 Casdoor 在线，首次登录时建立连接。
type OIDCManager struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	secret       []byte
	stateTTL     time.Duration

	mu       sync.Mutex
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	config   oauth2.Config
}

// NewOIDCManager 创建 OIDC 客户端（不发起网络请求）。
func NewOIDCManager(issuer, clientID, clientSecret, redirectURI string, stateTTL time.Duration) *OIDCManager {
	return &OIDCManager{
		issuer:       issuer,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		secret:       []byte(clientSecret),
		stateTTL:     stateTTL,
	}
}

// ensure 惰性加载 OIDC Provider（幂等，并发安全）。
func (m *OIDCManager) ensure(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.provider != nil {
		return nil
	}
	provider, err := oidc.NewProvider(ctx, m.issuer)
	if err != nil {
		return fmt.Errorf("连接 Casdoor OIDC Provider 失败: %w", err)
	}
	m.provider = provider
	m.config = oauth2.Config{
		ClientID:     m.clientID,
		ClientSecret: m.clientSecret,
		RedirectURL:  m.redirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	m.verifier = provider.Verifier(&oidc.Config{ClientID: m.clientID})
	return nil
}

// oidcState 为无状态回调状态（HMAC 签名），携带回调落点与 PKCE/Nonce 材料。
type oidcState struct {
	Redirect string `json:"redirect"`
	Verifier string `json:"verifier"`
	Nonce    string `json:"nonce"`
	Expires  int64  `json:"exp"`
}

// LoginURL 生成 Casdoor 登录跳转地址。
// redirect 为登录成功后回到 Velora 的路径（仅允许站内相对路径）。
func (m *OIDCManager) LoginURL(redirect string) (string, error) {
	if err := m.ensure(context.Background()); err != nil {
		return "", err
	}
	verifier := oauth2.GenerateVerifier()
	nonce, err := RandomToken(16)
	if err != nil {
		return "", err
	}
	state, err := m.encodeState(&oidcState{
		Redirect: redirect,
		Verifier: verifier,
		Nonce:    nonce,
		Expires:  time.Now().Add(m.stateTTL).Unix(),
	})
	if err != nil {
		return "", err
	}
	return m.config.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
	), nil
}

// Exchange 用回调 code 交换 token，校验 id_token（含 nonce）并获取用户信息。
func (m *OIDCManager) Exchange(ctx context.Context, code, stateToken string) (*CurrentUser, error) {
	if err := m.ensure(ctx); err != nil {
		return nil, err
	}
	state, err := m.decodeState(stateToken)
	if err != nil {
		return nil, err
	}
	if time.Now().Unix() > state.Expires {
		return nil, errors.New("OIDC state 已过期")
	}

	token, err := m.config.Exchange(ctx, code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		return nil, fmt.Errorf("OIDC token 交换失败: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("OIDC 响应缺少 id_token")
	}
	idToken, err := m.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token 校验失败: %w", err)
	}
	if idToken.Nonce != state.Nonce {
		return nil, errors.New("id_token nonce 校验失败")
	}

	// 用户信息：优先 OIDC UserInfo 端点，缺失字段回退 id_token claims。
	var claims map[string]any
	_ = idToken.Claims(&claims)

	user := &CurrentUser{
		ID:       idToken.Subject,
		Username: firstString(claims, "preferred_username", "name"),
		Email:    firstString(claims, "email"),
		Avatar:   firstString(claims, "picture"),
	}

	if info, err := m.provider.UserInfo(ctx, oauth2.StaticTokenSource(token)); err == nil {
		var uClaims map[string]any
		if err := info.Claims(&uClaims); err == nil {
			mergeClaims(user, uClaims)
		}
	} else if user.Username == "" {
		// UserInfo 失败且无兜底数据时视为登录失败，避免匿名会话。
		return nil, errors.New("OIDC UserInfo 获取失败")
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if user.Username == "" {
		user.Username = user.ID
	}
	return user, nil
}

// LoginWithPassword 通过 Casdoor OAuth2 Resource Owner Password 模式认证账号密码。
// Velora 仅做代理：密码只经 HTTPS 提交给 Casdoor token 端点，不落库、不存储；
// 返回的 id_token 做签名/issuer/audience/过期校验后解析用户信息。
// （password 模式由 Casdoor 签发的 id_token 不带 nonce，故此处不做 nonce 校验。）
func (m *OIDCManager) LoginWithPassword(ctx context.Context, username, password string) (*CurrentUser, error) {
	if err := m.ensure(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, errs.New(errs.CodeLoginFailed, http.StatusUnauthorized, "账号或密码不能为空")
	}

	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", m.clientID)
	form.Set("client_secret", m.clientSecret)
	form.Set("username", username)
	form.Set("password", password)
	form.Set("scope", "openid profile email")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.config.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("构造登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Casdoor 登录失败: %w", err)
	}
	defer resp.Body.Close()

	// Casdoor 同时可能返回标准 OAuth 错误或 Casdoor 包装错误（status/msg）。
	var payload struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		IDToken          string `json:"id_token"`
		Status           string `json:"status"`
		Msg              string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析 Casdoor 登录响应失败: %w", err)
	}
	if payload.Error != "" || payload.Status == "error" {
		msg := payload.Msg
		if msg == "" {
			msg = payload.ErrorDescription
		}
		if msg == "" {
			msg = payload.Error
		}
		return nil, errs.New(errs.CodeLoginFailed, http.StatusUnauthorized, "账号或密码错误")
	}
	if payload.IDToken == "" {
		return nil, errs.New(errs.CodeLoginFailed, http.StatusUnauthorized, "登录服务未返回有效凭证")
	}

	idToken, err := m.verifier.Verify(ctx, payload.IDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token 校验失败: %w", err)
	}

	var claims map[string]any
	_ = idToken.Claims(&claims)
	user := &CurrentUser{
		ID:       idToken.Subject,
		Username: firstString(claims, "preferred_username", "name"),
		Email:    firstString(claims, "email"),
		Avatar:   firstString(claims, "picture"),
	}
	mergeClaims(user, claims)
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if user.Username == "" {
		user.Username = user.ID
	}
	return user, nil
}

// mergeClaims 合并 UserInfo claims 到用户模型（UserInfo 优先级更高）。
func mergeClaims(u *CurrentUser, claims map[string]any) {
	if v := firstString(claims, "preferred_username", "name"); v != "" {
		u.Username = v
	}
	if v := firstString(claims, "display_name", "displayName"); v != "" {
		u.DisplayName = v
	}
	if v := firstString(claims, "email"); v != "" {
		u.Email = v
	}
	if v := firstString(claims, "picture", "avatar"); v != "" {
		u.Avatar = v
	}
	if v := firstString(claims, "organization", "org", "organizationName"); v != "" {
		u.Organization = v
	}
	if v := firstString(claims, "display_name", "displayName"); v != "" {
		u.DisplayName = v
	}
	u.Roles = mergeStrings(u.Roles, firstStringSlice(claims, "roles", "role"))
	u.Groups = mergeStrings(u.Groups, firstStringSlice(claims, "groups", "group"))
}

func firstString(claims map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := claims[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

func firstStringSlice(claims map[string]any, keys ...string) []string {
	for _, k := range keys {
		if v, ok := claims[k]; ok {
			switch t := v.(type) {
			case []any:
				out := make([]string, 0, len(t))
				for _, item := range t {
					// 兼容 Casdoor：roles 可能是字符串数组或对象数组（{"owner":...,"name":...}）。
					switch it := item.(type) {
					case string:
						if it != "" {
							out = append(out, it)
						}
					case map[string]any:
						if name, ok := it["name"].(string); ok && name != "" {
							out = append(out, name)
						}
					}
				}
				return out
			case []string:
				return t
			case string:
				if t != "" {
					return []string{t}
				}
			}
		}
	}
	return nil
}

func mergeStrings(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string{}, a...), b...) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// --- state 编解码 ---

func (m *OIDCManager) encodeState(s *oidcState) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	sig := m.signState(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (m *OIDCManager) decodeState(token string) (*oidcState, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("OIDC state 格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("OIDC state 解码失败")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("OIDC state 签名解码失败")
	}
	if !hmac.Equal(sig, m.signState(payload)) {
		return nil, errors.New("OIDC state 签名校验失败")
	}
	var s oidcState
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, errors.New("OIDC state 解析失败")
	}
	if time.Now().Unix() > s.Expires {
		return nil, errors.New("OIDC state 已过期")
	}
	return &s, nil
}

func (m *OIDCManager) signState(payload []byte) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}
