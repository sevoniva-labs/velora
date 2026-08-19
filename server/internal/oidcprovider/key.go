package oidcprovider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
)

// EnsureSigningKey 确保存在当前签名密钥：无任何 key 时生成；存在但超过轮换周期时新增。
// 幂等、并发安全（CREATE 唯一键冲突时忽略）。应在服务启动时调用。
func (s *Service) EnsureSigningKey(ctx context.Context) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&SigningKey{}).Count(&count).Error; err != nil {
		return errs.DB(err)
	}
	if count == 0 {
		_, err := s.generateKey(ctx)
		return err
	}
	// 检查是否需要轮换：最新 key 的 created_at 是否超过轮换周期
	var latest SigningKey
	if err := s.db.WithContext(ctx).Order("created_at DESC").First(&latest).Error; err != nil {
		return errs.DB(err)
	}
	if time.Since(latest.CreatedAt) > keyRotationInterval {
		_, err := s.generateKey(ctx)
		return err
	}
	return nil
}

// generateKey 生成新密钥对并入库（幂等：并发冲突时返回当前最新 key）。
func (s *Service) generateKey(ctx context.Context) (*SigningKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errs.Internal("生成 RSA 密钥失败", err)
	}
	kidBytes := make([]byte, 8)
	if _, err := rand.Read(kidBytes); err != nil {
		return nil, errs.Internal("生成 kid 失败", err)
	}
	key := &SigningKey{
		KID:        hex.EncodeToString(kidBytes),
		Alg:        "RS256",
		PublicPEM:  encodeRSAPublicPEM(&priv.PublicKey),
		PrivatePEM: encodeRSAPrivatePEM(priv),
		NotBefore:  time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(key).Error; err != nil {
		// 并发插入冲突：忽略（另一实例已创建），返回当前最新 key
		if strings.Contains(err.Error(), "SQLSTATE 23505") || strings.Contains(err.Error(), "duplicate key") {
			var latest SigningKey
			if err2 := s.db.WithContext(ctx).Order("created_at DESC").First(&latest).Error; err2 == nil {
				return &latest, nil
			}
		}
		return nil, errs.DB(err)
	}
	return key, nil
}

// currentSigningKey 返回当前签发密钥（最新）。
func (s *Service) currentSigningKey(ctx context.Context) (*SigningKey, error) {
	var key SigningKey
	err := s.db.WithContext(ctx).Order("created_at DESC").First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 兜底：自动生成
			return s.generateKey(ctx)
		}
		return nil, errs.DB(err)
	}
	return &key, nil
}

// findVerificationKey 按 kid 查找验证密钥（含宽限期内的旧 key）。
func (s *Service) findVerificationKey(ctx context.Context, kid string) (*SigningKey, error) {
	var key SigningKey
	err := s.db.WithContext(ctx).Where("kid = ?", kid).First(&key).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("JWT kid 不存在")
		}
		return nil, errs.DB(err)
	}
	return &key, nil
}

// JWKS 返回当前有效公钥列表（当前 + 宽限期内旧 key）。
func (s *Service) JWKS(ctx context.Context) (*JWKS, error) {
	var keys []SigningKey
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, errs.DB(err)
	}
	out := &JWKS{Keys: []JWK{}}
	for _, k := range keys {
		// 轮换后旧 key 在宽限期内仍发布（验证存量 token）
		if time.Since(k.CreatedAt) > keyRotationInterval+keyGracePeriod {
			continue
		}
		pub, err := parsePEMPublicKey(k.PublicPEM)
		if err != nil {
			continue
		}
		out.Keys = append(out.Keys, publicJWK(k.KID, pub))
	}
	return out, nil
}

// VerifyAccessToken 验证 access_token（按 kid 找 key）并返回 claims。
func (s *Service) VerifyAccessToken(ctx context.Context, token string) (*jwtClaims, error) {
	kid, err := extractKID(token)
	if err != nil {
		return nil, err
	}
	key, err := s.findVerificationKey(ctx, kid)
	if err != nil {
		return nil, err
	}
	pub, err := parsePEMPublicKey(key.PublicPEM)
	if err != nil {
		return nil, err
	}
	claims, err := verifyJWT(token, pub)
	if err != nil {
		return nil, err
	}
	// 吊销检查：access_jti 在 oidc_tokens 中且未吊销
	if err := s.checkNotRevoked(ctx, claims.JTI); err != nil {
		return nil, err
	}
	return claims, nil
}

// checkNotRevoked 校验 access_token 的 jti 未吊销（tokens 表存在且 revoked_at 为空）。
func (s *Service) checkNotRevoked(ctx context.Context, jti string) error {
	if jti == "" {
		return errors.New("access_token 缺少 jti")
	}
	var tok Token
	err := s.db.WithContext(ctx).Where("access_jti = ?", hashToken(jti)).First(&tok).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("access_token 无效")
		}
		return errs.DB(err)
	}
	if tok.RevokedAt != nil {
		return errors.New("access_token 已吊销")
	}
	return nil
}

// extractKID 从 JWT 头部提取 kid。
func extractKID(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("JWT 格式无效")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", errors.New("JWT header 解码失败")
	}
	var header struct {
		KID string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", errors.New("JWT header 解析失败")
	}
	if header.KID == "" {
		return "", errors.New("JWT 缺少 kid")
	}
	return header.KID, nil
}
