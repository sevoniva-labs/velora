package oidcprovider

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// jwtClaims 为 Velora 签发的 access_token / id_token claims。
type jwtClaims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Audience  []string `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
	JTI       string   `json:"jti,omitempty"`
	Nonce     string   `json:"nonce,omitempty"`
	Username  string   `json:"preferred_username,omitempty"`
	Email     string   `json:"email,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	Groups    []string `json:"groups,omitempty"`
}

// signJWT 用 RS256 签发 JWT。
func signJWT(key *SigningKey, claims jwtClaims) (string, error) {
	priv, err := parsePEMPrivateKey(key.PrivatePEM)
	if err != nil {
		return "", err
	}
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": key.KID}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	seg := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(payloadJSON)
	hashed := sha256.Sum256([]byte(seg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}
	return seg + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// verifyJWT 验证 JWT 签名与过期时间，返回 claims（用户信息）。
func verifyJWT(token string, pub *rsa.PublicKey) (*jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("JWT 格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("JWT payload 解码失败")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("JWT 签名解码失败")
	}
	hashed := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hashed[:], sig); err != nil {
		return nil, errors.New("JWT 签名校验失败")
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errors.New("JWT claims 解析失败")
	}
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("JWT 已过期")
	}
	return &claims, nil
}

// --- JWKS ---

// JWKS 为 /oidc/jwks 响应结构。
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK 为单个公钥条目。
type JWK struct {
	Kty string   `json:"kty"`
	Use string   `json:"use"`
	Kid string   `json:"kid"`
	Alg string   `json:"alg"`
	N   string   `json:"n"`
	E   string   `json:"e"`
}

// publicJWK 由 RSA 公钥生成 JWK。
func publicJWK(kid string, pub *rsa.PublicKey) JWK {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(bigEndianBytes(pub.E))
	return JWK{Kty: "RSA", Use: "sig", Kid: kid, Alg: "RS256", N: n, E: e}
}

// bigEndianBytes 把 uint 转为大端字节。
func bigEndianBytes(v int) []byte {
	if v == 0 {
		return []byte{0}
	}
	var out []byte
	for v > 0 {
		out = append([]byte{byte(v & 0xff)}, out...)
		v >>= 8
	}
	return out
}
