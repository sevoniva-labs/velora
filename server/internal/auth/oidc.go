package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCManager 封装与 Casdoor 的 OIDC Authorization Code + PKCE 交互。
type OIDCManager struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	config   oauth2.Config
	secret   []byte
	stateTTL time.Duration
}

// NewOIDCManager 基于 Casdoor Issuer 构建 OIDC 客户端。
func NewOIDCManager(ctx context.Context, issuer, clientID, clientSecret, redirectURI string, stateTTL time.Duration) (*OIDCManager, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("连接 Casdoor OIDC Provider 失败: %w", err)
	}
	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &OIDCManager{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		config:   config,
		secret:   []byte(clientSecret),
		stateTTL: stateTTL,
	}, nil
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
					if s, ok := item.(string); ok && s != "" {
						out = append(out, s)
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
