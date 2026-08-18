package mail

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// CredentialCipher 提供邮箱凭证的 AES-256-GCM 加解密。
// 密钥来源：MAIL_CREDENTIAL_KEY（base64 32 字节）；开发环境缺省时
// 由 SESSION_SECRET 派生（仅本地开发便利，生产必须显式配置独立密钥）。
// 约束：密钥与密文不得同库存储；日志禁止输出明文凭证。
type CredentialCipher struct {
	key []byte
}

// NewCredentialCipher 构造加解密器。keyB64 为空时用 fallbackSecret 派生。
func NewCredentialCipher(keyB64, fallbackSecret string) (*CredentialCipher, error) {
	var key []byte
	if keyB64 != "" {
		raw, err := base64.StdEncoding.DecodeString(keyB64)
		if err != nil || len(raw) != 32 {
			return nil, fmt.Errorf("MAIL_CREDENTIAL_KEY 必须为 base64 编码的 32 字节密钥")
		}
		key = raw
	} else {
		if fallbackSecret == "" {
			return nil, fmt.Errorf("MAIL_CREDENTIAL_KEY 未配置且无派生密钥来源")
		}
		sum := sha256.Sum256([]byte(fallbackSecret + "|velora-mail-credential"))
		key = sum[:]
	}
	return &CredentialCipher{key: key}, nil
}

// Encrypt 加密明文凭证，输出 base64(nonce | ciphertext)。
func (c *CredentialCipher) Encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt 解密凭证密文。
func (c *CredentialCipher) Decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("凭证密文格式错误")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("凭证密文长度异常")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("凭证解密失败（密钥可能已变更）")
	}
	return string(plain), nil
}
