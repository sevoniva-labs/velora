package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/emmansun/gmsm/sm3"
	"github.com/emmansun/gmsm/sm4"
)

type Provider interface {
	Name() string
	KeyVersion() string
	Hash(data []byte) []byte
	Encrypt(plaintext, aad []byte) (string, error)
	Decrypt(encoded string, aad []byte) ([]byte, error)
}

func New(name string, rawKey string, versions ...string) (Provider, error) {
	version := "v1"
	if len(versions) > 0 && versions[0] != "" {
		version = versions[0]
	}
	return NewKeyring(name, map[string]string{version: rawKey}, version)
}

func normalizeKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("VELORA_CRYPTO_KEY is required")
	}
	if b, e := base64.StdEncoding.DecodeString(raw); e == nil && len(b) >= 32 {
		return b[:32], nil
	}
	if len([]byte(raw)) >= 32 {
		sum := sha256.Sum256([]byte(raw))
		return sum[:], nil
	}
	return nil, errors.New("VELORA_CRYPTO_KEY must contain at least 32 bytes or be base64 encoding of at least 32 bytes")
}

type standard struct {
	key     []byte
	version string
}

func (s *standard) Name() string            { return "standard" }
func (s *standard) KeyVersion() string      { return s.version }
func (s *standard) Hash(data []byte) []byte { x := sha256.Sum256(data); return x[:] }
func (s *standard) Encrypt(p, aad []byte) (string, error) {
	block, e := aes.NewCipher(s.key)
	if e != nil {
		return "", e
	}
	return seal(s.version, block, p, aad)
}
func (s *standard) Decrypt(v string, aad []byte) ([]byte, error) {
	block, e := aes.NewCipher(s.key)
	if e != nil {
		return nil, e
	}
	return open(s.version, block, v, aad)
}

type gm struct {
	key     []byte
	version string
}

func (g *gm) Name() string            { return "gm" }
func (g *gm) KeyVersion() string      { return g.version }
func (g *gm) Hash(data []byte) []byte { x := sm3.Sum(data); return x[:] }
func (g *gm) Encrypt(p, aad []byte) (string, error) {
	block, e := sm4.NewCipher(g.key)
	if e != nil {
		return "", e
	}
	return seal(g.version, block, p, aad)
}
func (g *gm) Decrypt(v string, aad []byte) ([]byte, error) {
	block, e := sm4.NewCipher(g.key)
	if e != nil {
		return nil, e
	}
	return open(g.version, block, v, aad)
}

func seal(version string, block cipher.Block, p, aad []byte) (string, error) {
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return "", e
	}
	out := gcm.Seal(nonce, nonce, p, aad)
	return version + "." + base64.RawURLEncoding.EncodeToString(out), nil
}
func open(version string, block cipher.Block, v string, aad []byte) ([]byte, error) {
	encoded := v
	if i := strings.IndexByte(v, '.'); i >= 0 {
		if v[:i] != version {
			return nil, fmt.Errorf("ciphertext key version %q is not active version %q", v[:i], version)
		}
		encoded = v[i+1:]
	}
	raw, e := base64.RawURLEncoding.DecodeString(encoded)
	if e != nil {
		return nil, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	n := gcm.NonceSize()
	if len(raw) < n {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, raw[:n], raw[n:], aad)
}
