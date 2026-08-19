package oidcprovider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// newTestService 用内存 sqlite 构建 Service（无外部依赖）。
func newTestService(t *testing.T) *Service {
	t.Helper()
	return newTestServiceWithDB(t)
}

func newTestServiceWithDB(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// AutoMigrate 建表（与生产迁移 0004 保持同构）
	require.NoError(t, db.AutoMigrate(&Client{}, &AuthCode{}, &Token{}, &SigningKey{}))
	return NewService(db)
}

func TestComputeS256(t *testing.T) {
	// 与标准库计算对比
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	expect := base64.RawURLEncoding.EncodeToString(sum[:])
	got := computeS256(verifier)
	assert.Equal(t, expect, got)
}

func TestRedirectAllowed(t *testing.T) {
	cl := &Client{RedirectURIsRaw: `["https://app.example.com/callback","http://localhost:3000/auth"]`}
	assert.True(t, redirectAllowed(cl, "https://app.example.com/callback"))
	assert.True(t, redirectAllowed(cl, "http://localhost:3000/auth"))
	assert.False(t, redirectAllowed(cl, "https://evil.example.com/callback"), "白名单外应拒绝")
	assert.False(t, redirectAllowed(cl, "https://app.example.com/callback.evil"), "前缀混淆应拒绝")
	assert.False(t, redirectAllowed(cl, ""), "空 redirect 应拒绝")
	assert.False(t, redirectAllowed(cl, "javascript:alert(1)"), "非 http(s) 应拒绝")
}

func TestExtractKID(t *testing.T) {
	kid, err := extractKID("eyJraWQiOiJhYmMifQ.abc.xyz")
	require.NoError(t, err)
	assert.Equal(t, "abc", kid)
}

func TestIssueAndExchangeCode(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	cl, secret, err := s.CreateClient(ctx, 1, []string{"https://app.example.com/cb"}, []string{GrantTypeAuthorizationCode})
	require.NoError(t, err)
	assert.NotEmpty(t, cl.ClientID)
	assert.NotEmpty(t, secret)
	assert.NotEmpty(t, cl.ClientSecretHash)

	user := &auth.CurrentUser{ID: "u1", Username: "alice", Email: "alice@example.com", Roles: []string{"velora_admin"}}
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	code, err := s.IssueCode(ctx, cl, user, "https://app.example.com/cb", "openid profile", computeS256(verifier), "S256", "nonce123")
	require.NoError(t, err)
	assert.NotEmpty(t, code)

	// 正确 verifier → 成功
	tok, access, refresh, err := s.ExchangeCode(ctx, cl, code, "https://app.example.com/cb", verifier)
	require.NoError(t, err)
	assert.NotNil(t, tok)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)

	// code 一次性：再次使用失败
	_, _, _, err = s.ExchangeCode(ctx, cl, code, "https://app.example.com/cb", verifier)
	require.Error(t, err)
	var e *errs.Error
	if ok := assert.ErrorAs(t, err, &e); ok {
		assert.Equal(t, errs.CodeOIDCProviderInvalidGrant, e.Code)
	}
}

func TestExchangeCodeWrongPKCE(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	cl, _, err := s.CreateClient(ctx, 1, []string{"https://app.example.com/cb"}, nil)
	require.NoError(t, err)
	user := &auth.CurrentUser{ID: "u1"}
	verifier := "correct-verifier-123456789012345678901234567890"
	code, err := s.IssueCode(ctx, cl, user, "https://app.example.com/cb", "openid", computeS256(verifier), "S256", "")
	require.NoError(t, err)

	_, _, _, err = s.ExchangeCode(ctx, cl, code, "https://app.example.com/cb", "wrong-verifier-9876543210")
	require.Error(t, err)
	var e *errs.Error
	if ok := assert.ErrorAs(t, err, &e); ok {
		assert.Equal(t, errs.CodeOIDCProviderInvalidGrant, e.Code)
	}
}

func TestRefreshRotation(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	cl, _, err := s.CreateClient(ctx, 1, []string{"https://app.example.com/cb"}, nil)
	require.NoError(t, err)
	user := &auth.CurrentUser{ID: "u1", Username: "bob"}
	verifier := "verifier-abcdefghijklmnopqrstuvwxyz012345"
	code, err := s.IssueCode(ctx, cl, user, "https://app.example.com/cb", "openid", computeS256(verifier), "S256", "")
	require.NoError(t, err)
	_, _, refresh, err := s.ExchangeCode(ctx, cl, code, "https://app.example.com/cb", verifier)
	require.NoError(t, err)

	// 第一次刷新成功
	_, _, newRefresh, err := s.RefreshToken(ctx, cl, refresh)
	require.NoError(t, err)
	assert.NotEmpty(t, newRefresh)
	// 旧 refresh 已轮换 → 失败
	_, _, _, err = s.RefreshToken(ctx, cl, refresh)
	require.Error(t, err)
	// 新 refresh 可用
	_, _, _, err = s.RefreshToken(ctx, cl, newRefresh)
	require.NoError(t, err)
}

func TestJWKSAndVerify(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	require.NoError(t, s.EnsureSigningKey(ctx))

	jwks, err := s.JWKS(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, jwks.Keys)
	kid := jwks.Keys[0].Kid

	// 用内部 key 直接签发再验证
	key, err := s.currentSigningKey(ctx)
	require.NoError(t, err)
	access, err := signJWT(key, jwtClaims{
		Issuer: "https://velora.example.com", Subject: "u1",
		Audience: []string{"c1"}, ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt: time.Now().Unix(), JTI: "jti1", Username: "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, kid, key.KID)

	claims, err := s.VerifyAccessToken(ctx, access)
	require.Error(t, err, "jti 未在 tokens 表注册应失败")
	_ = claims
}

func TestClientAuth(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()
	cl, secret, err := s.CreateClient(ctx, 1, []string{"https://app.example.com/cb"}, nil)
	require.NoError(t, err)

	got, err := s.authenticateClient(ctx, cl.ClientID, secret)
	require.NoError(t, err)
	assert.Equal(t, cl.ClientID, got.ClientID)

	_, err = s.authenticateClient(ctx, cl.ClientID, "wrong-secret")
	require.Error(t, err)
	var e *errs.Error
	if ok := assert.ErrorAs(t, err, &e); ok {
		assert.Equal(t, errs.CodeOIDCProviderInvalidClient, e.Code)
	}
}
