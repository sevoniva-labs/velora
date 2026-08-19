// Package oidcprovider 实现 Velora 作为 OIDC Provider（IdP）的能力。
//
// 目标：第三方应用把 Velora 配为 OIDC IdP（issuer / client_id / client_secret / 回调），
// 用户点击应用 SSO 登录时，跳转的是 **Velora 登录页**（未登录）或静默放行（已登录），
// Casdoor 完全隐藏在 Velora 之后。
//
// 安全要点（对齐 production-plan.md Phase B）：
//   - code / refresh token 只存 SHA-256 哈希；client_secret 只存 bcrypt 哈希
//   - PKCE S256 强制；redirect_uri 严格白名单（复用 auth 的防 Open Redirect 经验）
//   - access_token 用 RS256 JWT（短 TTL + jti）；refresh_token 落库可吊销
//   - 签名密钥 90 天轮换、30 天宽限期（旧 key 仍可验证）
package oidcprovider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// --- 常量 ---

const (
	// GrantTypeAuthorizationCode 授权码模式。
	GrantTypeAuthorizationCode = "authorization_code"
	// GrantTypeRefreshToken 刷新令牌模式。
	GrantTypeRefreshToken = "refresh_token"

	// codeTTL 授权码有效期。
	codeTTL = 10 * time.Minute
	// accessTokenTTL access_token 有效期（JWT）。
	accessTokenTTL = 1 * time.Hour
	// refreshTokenTTL refresh_token 有效期。
	refreshTokenTTL = 30 * 24 * time.Hour
	// keyRotationInterval 签名密钥轮换周期。
	keyRotationInterval = 90 * 24 * time.Hour
	// keyGracePeriod 旧密钥宽限期（轮换后旧 key 仍可用于验证）。
	keyGracePeriod = 30 * 24 * time.Hour
)

// --- 实体 ---

// Client 为 OIDC 客户端（第三方应用）。
type Client struct {
	ID                      uint64    `gorm:"column:id;primaryKey" json:"id"`
	ApplicationID           uint64    `gorm:"column:application_id" json:"applicationId"`
	ClientID                string    `gorm:"column:client_id" json:"clientId"`
	ClientSecretHash        string    `gorm:"column:client_secret_hash" json:"-"`
	RedirectURIsRaw         string    `gorm:"column:redirect_uris" json:"-"`
	GrantTypesRaw           string    `gorm:"column:grant_types" json:"-"`
	ScopesRaw               string    `gorm:"column:scopes" json:"-"`
	TokenEndpointAuthMethod string    `gorm:"column:token_endpoint_auth_method" json:"-"`
	Enabled                 bool      `gorm:"column:enabled" json:"enabled"`
	CreatedAt               time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt               time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名。
func (Client) TableName() string { return "oidc_clients" }

// RedirectURIs 解析白名单。
func (c *Client) RedirectURIs() []string {
	var out []string
	_ = json.Unmarshal([]byte(c.RedirectURIsRaw), &out)
	return out
}

// MarshalJSON 定制导出：把 JSON 数组字段展开为可直接使用的结构。
func (c Client) MarshalJSON() ([]byte, error) {
	type alias Client
	return json.Marshal(struct {
		alias
		RedirectURIs []string `json:"redirectUris"`
		GrantTypes   []string `json:"grantTypes"`
		Scopes       []string `json:"scopes"`
	}{
		alias:        alias(c),
		RedirectURIs: c.RedirectURIs(),
		GrantTypes:   c.GrantTypes(),
		Scopes:       c.Scopes(),
	})
}

// GrantTypes 解析授权类型。
func (c *Client) GrantTypes() []string {
	var out []string
	_ = json.Unmarshal([]byte(c.GrantTypesRaw), &out)
	return out
}

// Scopes 解析 scopes。
func (c *Client) Scopes() []string {
	var out []string
	_ = json.Unmarshal([]byte(c.ScopesRaw), &out)
	return out
}

// HasGrant 判断是否支持某授权类型。
func (c *Client) HasGrant(g string) bool {
	for _, v := range c.GrantTypes() {
		if v == g {
			return true
		}
	}
	return false
}

// AuthCode 为一次性授权码（库中只存哈希）。
type AuthCode struct {
	ID                  uint64    `gorm:"column:id;primaryKey" json:"-"`
	CodeHash            string    `gorm:"column:code_hash" json:"-"`
	ClientID            string    `gorm:"column:client_id" json:"-"`
	UserID              string    `gorm:"column:user_id" json:"-"`
	RedirectURI         string    `gorm:"column:redirect_uri" json:"-"`
	Scope               string    `gorm:"column:scope" json:"-"`
	CodeChallenge       string    `gorm:"column:code_challenge" json:"-"`
	CodeChallengeMethod string    `gorm:"column:code_challenge_method" json:"-"`
	Nonce               string    `gorm:"column:nonce" json:"-"`
	ExpiresAt           time.Time `gorm:"column:expires_at" json:"-"`
	Used                bool      `gorm:"column:used" json:"-"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"-"`
}

// TableName 指定表名。
func (AuthCode) TableName() string { return "oidc_auth_codes" }

// Token 为签发的令牌记录（refresh 落库，access 为 JWT）。
type Token struct {
	ID          uint64     `gorm:"column:id;primaryKey" json:"-"`
	ClientID    string     `gorm:"column:client_id" json:"-"`
	UserID      string     `gorm:"column:user_id" json:"-"`
	AccessJTI   string     `gorm:"column:access_jti" json:"-"`
	RefreshHash string     `gorm:"column:refresh_hash" json:"-"`
	Scope       string     `gorm:"column:scope" json:"-"`
	ExpiresAt   time.Time  `gorm:"column:expires_at" json:"-"`
	RevokedAt   *time.Time `gorm:"column:revoked_at" json:"-"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"-"`
}

// TableName 指定表名。
func (Token) TableName() string { return "oidc_tokens" }

// SigningKey 为签名密钥对（RS256）。
type SigningKey struct {
	KID        string     `gorm:"column:kid;primaryKey" json:"kid"`
	Alg        string     `gorm:"column:alg" json:"alg"`
	PublicPEM  string     `gorm:"column:public_pem" json:"-"`
	PrivatePEM string     `gorm:"column:private_pem" json:"-"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"-"`
	NotBefore  time.Time  `gorm:"column:not_before" json:"-"`
	ExpiresAt  *time.Time `gorm:"column:expires_at" json:"-"`
}

// TableName 指定表名。
func (SigningKey) TableName() string { return "oidc_keys" }

// --- Service ---

// Service 为 OIDC Provider 核心服务。
type Service struct {
	db        *gorm.DB
	issuerURL string
	// userSnapshot 从 Velora 会话快照加载用户（Phase B6 由组装层注入）。
	// 返回 auth.CurrentUser 填充 Username/Email/Roles/Groups。
	userSnapshot func(ctx context.Context, userID string) (*auth.CurrentUser, error)
}

// NewService 创建服务。
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// SetUserSnapshot 注入用户快照加载器（Phase B6 会话表实现）。
func (s *Service) SetUserSnapshot(fn func(ctx context.Context, userID string) (*auth.CurrentUser, error)) {
	s.userSnapshot = fn
}

// ---------- 客户端管理 ----------

// CreateClient 创建 OIDC 客户端，返回明文 client_secret（仅此一次展示）。
// redirectURIs 为白名单（json 数组字符串或 []string）。
func (s *Service) CreateClient(ctx context.Context, applicationID uint64, redirectURIs []string, grantTypes []string) (*Client, string, error) {
	clientID, err := randomToken(16)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomToken(32)
	if err != nil {
		return nil, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", errs.Internal("生成 client_secret 失败", err)
	}
	if len(grantTypes) == 0 {
		grantTypes = []string{GrantTypeAuthorizationCode}
	}
	redirectJSON, _ := json.Marshal(redirectURIs)
	grantJSON, _ := json.Marshal(grantTypes)
	scopeJSON, _ := json.Marshal([]string{"openid", "profile", "email"})

	cl := &Client{
		ApplicationID:           applicationID,
		ClientID:                clientID,
		ClientSecretHash:        string(hash),
		RedirectURIsRaw:         string(redirectJSON),
		GrantTypesRaw:           string(grantJSON),
		ScopesRaw:               string(scopeJSON),
		TokenEndpointAuthMethod: "client_secret_post",
		Enabled:                 true,
	}
	if err := s.db.WithContext(ctx).Create(cl).Error; err != nil {
		return nil, "", errs.DB(err)
	}
	return cl, secret, nil
}

// GetClientByID 按 client_id 查询启用中的客户端。
func (s *Service) GetClientByID(ctx context.Context, clientID string) (*Client, error) {
	var cl Client
	err := s.db.WithContext(ctx).Where("client_id = ?", clientID).First(&cl).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeOIDCProviderClientNotFound, 400, "OIDC 客户端不存在")
		}
		return nil, errs.DB(err)
	}
	if !cl.Enabled {
		return nil, errs.New(errs.CodeOIDCProviderClientNotFound, 400, "OIDC 客户端已禁用")
	}
	return &cl, nil
}

// ListClientsByApplication 按门户应用列出客户端（管理后台展示）。
func (s *Service) ListClientsByApplication(ctx context.Context, applicationID uint64) ([]Client, error) {
	var list []Client
	if err := s.db.WithContext(ctx).Where("application_id = ?", applicationID).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errs.DB(err)
	}
	return list, nil
}

// SetClientEnabled 启用/禁用客户端。
func (s *Service) SetClientEnabled(ctx context.Context, id uint64, enabled bool) error {
	res := s.db.WithContext(ctx).Model(&Client{}).Where("id = ?", id).Update("enabled", enabled)
	if res.Error != nil {
		return errs.DB(res.Error)
	}
	return nil
}

// RevokeClientByClientID 按 client_id 吊销（禁用）客户端，并吊销其全部令牌。
func (s *Service) RevokeClientByClientID(ctx context.Context, clientID string) error {
	res := s.db.WithContext(ctx).Model(&Client{}).Where("client_id = ?", clientID).Update("enabled", false)
	if res.Error != nil {
		return errs.DB(res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeOIDCProviderClientNotFound, 404, "OIDC 客户端不存在")
	}
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&Token{}).
		Where("client_id = ? AND revoked_at IS NULL", clientID).
		Update("revoked_at", now).Error; err != nil {
		return errs.DB(err)
	}
	return nil
}

// authenticateClient 校验 client_id + client_secret（token 端点 Basic 或 body）。
func (s *Service) authenticateClient(ctx context.Context, clientID, secret string) (*Client, error) {
	cl, err := s.GetClientByID(ctx, clientID)
	if err != nil {
		return nil, errs.New(errs.CodeOIDCProviderInvalidClient, 401, "客户端认证失败")
	}
	if bcrypt.CompareHashAndPassword([]byte(cl.ClientSecretHash), []byte(secret)) != nil {
		return nil, errs.New(errs.CodeOIDCProviderInvalidClient, 401, "客户端认证失败")
	}
	return cl, nil
}

// ---------- 授权码 ----------

// IssueCode 为用户签发一次性授权码（已通过会话校验后调用）。
func (s *Service) IssueCode(ctx context.Context, cl *Client, user *auth.CurrentUser, redirectURI, scope, challenge, challengeMethod, nonce string) (string, error) {
	code, err := randomToken(32)
	if err != nil {
		return "", err
	}
	rec := &AuthCode{
		CodeHash:            hashToken(code),
		ClientID:            cl.ClientID,
		UserID:              user.ID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		CodeChallenge:       challenge,
		CodeChallengeMethod: challengeMethod,
		Nonce:               nonce,
		ExpiresAt:           time.Now().Add(codeTTL),
	}
	if err := s.db.WithContext(ctx).Create(rec).Error; err != nil {
		return "", errs.DB(err)
	}
	return code, nil
}

// ExchangeCode 用授权码换 token（校验 PKCE + 一次性 + 过期）。
func (s *Service) ExchangeCode(ctx context.Context, cl *Client, code, redirectURI, verifier string) (*Token, string, string, error) {
	rec := &AuthCode{}
	err := s.db.WithContext(ctx).Where("code_hash = ?", hashToken(code)).First(rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "授权码无效")
		}
		return nil, "", "", errs.DB(err)
	}
	if rec.Used || time.Now().After(rec.ExpiresAt) {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "授权码已使用或已过期")
	}
	if rec.ClientID != cl.ClientID {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "授权码与客户端不匹配")
	}
	if redirectURI != "" && rec.RedirectURI != redirectURI {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "redirect_uri 不匹配")
	}
	// PKCE 校验：S256 强制
	if rec.CodeChallenge == "" {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "缺少 PKCE challenge")
	}
	computed := computeS256(verifier)
	if computed != rec.CodeChallenge {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "PKCE 校验失败")
	}

	// 标记已用（一次性）
	if err := s.db.WithContext(ctx).Model(rec).Update("used", true).Error; err != nil {
		return nil, "", "", errs.DB(err)
	}

	// 签发 token
	return s.issueTokens(ctx, cl, rec.UserID, rec.Scope, rec.Nonce)
}

// issueTokens 签发 access(JWT) + refresh 并落库。
func (s *Service) issueTokens(ctx context.Context, cl *Client, userID, scope, nonce string) (*Token, string, string, error) {
	key, err := s.currentSigningKey(ctx)
	if err != nil {
		return nil, "", "", err
	}
	jti, err := randomToken(16)
	if err != nil {
		return nil, "", "", err
	}
	refresh, err := randomToken(32)
	if err != nil {
		return nil, "", "", err
	}

	user, err := s.loadUser(ctx, userID)
	if err != nil {
		return nil, "", "", err
	}

	accessToken, err := signJWT(key, jwtClaims{
		Issuer:    s.issuer(),
		Subject:   userID,
		Audience:  []string{cl.ClientID},
		ExpiresAt: time.Now().Add(accessTokenTTL).Unix(),
		IssuedAt:  time.Now().Unix(),
		JTI:       jti,
		Nonce:     nonce,
		Username:  user.Username,
		Email:     user.Email,
		Roles:     user.Roles,
		Groups:    user.Groups,
	})
	if err != nil {
		return nil, "", "", errs.Internal("签发 access_token 失败", err)
	}

	tok := &Token{
		ClientID:    cl.ClientID,
		UserID:      userID,
		AccessJTI:   hashToken(jti),
		RefreshHash: hashToken(refresh),
		Scope:       scope,
		ExpiresAt:   time.Now().Add(refreshTokenTTL),
	}
	if err := s.db.WithContext(ctx).Create(tok).Error; err != nil {
		return nil, "", "", errs.DB(err)
	}
	return tok, accessToken, refresh, nil
}

// RefreshToken 用 refresh_token 换取新 access_token（校验未吊销 + 未过期）。
func (s *Service) RefreshToken(ctx context.Context, cl *Client, refresh string) (*Token, string, string, error) {
	tok := &Token{}
	err := s.db.WithContext(ctx).Where("refresh_hash = ?", hashToken(refresh)).First(tok).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "refresh_token 无效")
		}
		return nil, "", "", errs.DB(err)
	}
	if tok.RevokedAt != nil || time.Now().After(tok.ExpiresAt) {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "refresh_token 已吊销或已过期")
	}
	if tok.ClientID != cl.ClientID {
		return nil, "", "", errs.New(errs.CodeOIDCProviderInvalidGrant, 400, "refresh_token 与客户端不匹配")
	}
	// 轮换 refresh_token：旧记录吊销，签发新记录（防重放）。
	if err := s.db.WithContext(ctx).Model(tok).Update("revoked_at", time.Now()).Error; err != nil {
		return nil, "", "", errs.DB(err)
	}
	key, err := s.currentSigningKey(ctx)
	if err != nil {
		return nil, "", "", err
	}
	jti, err := randomToken(16)
	if err != nil {
		return nil, "", "", err
	}
	newRefresh, err := randomToken(32)
	if err != nil {
		return nil, "", "", err
	}
	user, err := s.loadUser(ctx, tok.UserID)
	if err != nil {
		return nil, "", "", err
	}
	accessToken, err := signJWT(key, jwtClaims{
		Issuer:    s.issuer(),
		Subject:   tok.UserID,
		Audience:  []string{cl.ClientID},
		ExpiresAt: time.Now().Add(accessTokenTTL).Unix(),
		IssuedAt:  time.Now().Unix(),
		JTI:       jti,
		Username:  user.Username,
		Email:     user.Email,
		Roles:     user.Roles,
		Groups:    user.Groups,
	})
	if err != nil {
		return nil, "", "", errs.Internal("签发 access_token 失败", err)
	}
	newTok := &Token{
		ClientID:    cl.ClientID,
		UserID:      tok.UserID,
		AccessJTI:   hashToken(jti),
		RefreshHash: hashToken(newRefresh),
		Scope:       tok.Scope,
		ExpiresAt:   time.Now().Add(refreshTokenTTL),
	}
	if err := s.db.WithContext(ctx).Create(newTok).Error; err != nil {
		return nil, "", "", errs.DB(err)
	}
	return newTok, accessToken, newRefresh, nil
}

// RevokeUserTokens 吊销用户全部令牌（强制下线 / 改密时调用）。
func (s *Service) RevokeUserTokens(ctx context.Context, userID string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&Token{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// loadUser 从用户快照恢复用户（roles/groups 来自登录时缓存的会话，不实时查 Casdoor）。
func (s *Service) loadUser(ctx context.Context, userID string) (*auth.CurrentUser, error) {
	if s.userSnapshot != nil {
		if u, err := s.userSnapshot(ctx, userID); err == nil && u != nil {
			return u, nil
		}
	}
	// 回退最小用户（无快照时只保证 sub 正确）。
	return &auth.CurrentUser{ID: userID}, nil
}

// ---------- 密钥管理 ----------

// issuer 返回 Velora OIDC issuer 地址（由 handler 注入，见 SetIssuer）。
func (s *Service) issuer() string { return s.issuerURL }

// SetIssuer 设置对外 issuer 地址（如 https://velora.example.com）。
func (s *Service) SetIssuer(u string) { s.issuerURL = strings.TrimRight(u, "/") }

// ---------- 工具 ----------

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", errs.Internal("生成随机 token 失败", err)
	}
	return hex.EncodeToString(buf), nil
}

func computeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64RawURL(sum[:])
}

// base64RawURL 无填充 base64url 编码。
func base64RawURL(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var sb strings.Builder
	for i := 0; i < len(b); i += 3 {
		var n uint32
		remain := len(b) - i
		if remain >= 3 {
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
		} else if remain == 2 {
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8
		} else {
			n = uint32(b[i]) << 16
		}
		sb.WriteByte(alphabet[(n>>18)&63])
		sb.WriteByte(alphabet[(n>>12)&63])
		if remain >= 2 {
			sb.WriteByte(alphabet[(n>>6)&63])
		}
		if remain >= 3 {
			sb.WriteByte(alphabet[n&63])
		}
	}
	return sb.String()
}

// parsePEMPrivateKey 解析 RSA 私钥 PEM。
func parsePEMPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("私钥 PEM 解析失败")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// parsePEMPublicKey 解析 RSA 公钥 PEM。
func parsePEMPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("公钥 PEM 解析失败")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("非 RSA 公钥")
	}
	return rsaPub, nil
}

// encodeRSAPublicPEM 编码 RSA 公钥为 PKIX PEM。
func encodeRSAPublicPEM(pub *rsa.PublicKey) string {
	der, _ := x509.MarshalPKIXPublicKey(pub)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// encodeRSAPrivatePEM 编码 RSA 私钥为 PKCS1 PEM。
func encodeRSAPrivatePEM(priv *rsa.PrivateKey) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}))
}
