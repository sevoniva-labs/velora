// Package auth 提供 Casdoor OIDC 集成、会话管理与当前用户上下文。
//
// 设计边界：
//   - Casdoor 负责身份认证；Velora 只消费 OIDC 结果并建立自己的会话
//   - Velora 永不直接查询 Casdoor 数据库
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SessionCookieName 为会话 Cookie 名（HttpOnly）。
const SessionCookieName = "velora_session"

// CSRFCookieName 为 CSRF 双提交 Cookie 名（前端写请求需回传 X-CSRF-Token）。
const CSRFCookieName = "velora_csrf"

// Session 为 Velora 会话负载（基于 Casdoor OIDC 结果构建）。
type Session struct {
	UserID       string    `json:"uid"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	Email        string    `json:"email"`
	Avatar       string    `json:"avatar"`
	Organization string    `json:"organization"`
	Roles        []string  `json:"roles"`
	Groups       []string  `json:"groups"`
	IssuedAt     time.Time `json:"iat"`
	ExpiresAt    time.Time `json:"exp"`
}

// Expired 判断会话是否过期。
func (s *Session) Expired() bool {
	return time.Now().After(s.ExpiresAt)
}

// CurrentUser 为请求上下文中的当前用户（前端 /api/v1/me 返回结构）。
type CurrentUser struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	DisplayName  string   `json:"displayName"`
	Email        string   `json:"email"`
	Avatar       string   `json:"avatar"`
	Organization string   `json:"organization"`
	Roles        []string `json:"roles"`
	Groups       []string `json:"groups"`
}

// ToCurrentUser 由 Session 派生面向 API 的用户视图。
func (s *Session) ToCurrentUser() *CurrentUser {
	return &CurrentUser{
		ID:           s.UserID,
		Username:     s.Username,
		DisplayName:  s.DisplayName,
		Email:        s.Email,
		Avatar:       s.Avatar,
		Organization: s.Organization,
		Roles:        append([]string(nil), s.Roles...),
		Groups:       append([]string(nil), s.Groups...),
	}
}

// IsAdmin 判断用户是否持有 Velora 管理员角色。
func (u *CurrentUser) IsAdmin(adminRole string) bool {
	for _, r := range u.Roles {
		if r == adminRole {
			return true
		}
	}
	return false
}

// SessionStore 负责会话 Cookie 的签发、校验与销毁（HMAC-SHA256 签名，防篡改）。
type SessionStore struct {
	secret []byte
	ttl    time.Duration
	secure bool
	domain string
}

// NewSessionStore 创建会话存储。secret 至少 32 字节。
func NewSessionStore(secret string, ttl time.Duration, secure bool, domain string) (*SessionStore, error) {
	if len(secret) < 32 {
		return nil, errors.New("SESSION_SECRET 长度不足 32 字节")
	}
	return &SessionStore{secret: []byte(secret), ttl: ttl, secure: secure, domain: domain}, nil
}

// NewSession 创建新会话。
func (s *SessionStore) NewSession(user *CurrentUser) *Session {
	now := time.Now()
	return &Session{
		UserID:       user.ID,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
		Avatar:       user.Avatar,
		Organization: user.Organization,
		Roles:        append([]string(nil), user.Roles...),
		Groups:       append([]string(nil), user.Groups...),
		IssuedAt:     now,
		ExpiresAt:    now.Add(s.ttl),
	}
}

// Encode 把会话编码为「payload.signature」形式（均 base64url，无状态）。
func (s *SessionStore) Encode(session *Session) (string, error) {
	payload, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	sig := s.sign(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Decode 校验签名并解析会话。
func (s *SessionStore) Decode(token string) (*Session, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("会话格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("会话负载解码失败")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("会话签名解码失败")
	}
	if !hmac.Equal(sig, s.sign(payload)) {
		return nil, errors.New("会话签名校验失败")
	}
	var session Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, errors.New("会话解析失败")
	}
	if session.Expired() {
		return nil, errors.New("会话已过期")
	}
	return &session, nil
}

func (s *SessionStore) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

// CookieOptions 返回会话 Cookie 的公共属性。
func (s *SessionStore) CookieOptions() (path string, maxAge int, secure bool, domain string) {
	return "/", int(s.ttl.Seconds()), s.secure, s.domain
}

// RandomToken 生成随机 token（CSRF / OIDC state 使用）。
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
