package credentialhandoff

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	appcrypto "github.com/sevoniva-labs/velora/server/internal/platform/security/crypto"
)

const (
	defaultTTL = 5 * time.Minute
	aadValue   = "velora:credential-handoff:v1"
)

var (
	ErrUnavailable  = errors.New("credential handoff is unavailable")
	ErrInvalidToken = errors.New("enrollment token is invalid or expired")
)

type Bundle struct {
	ApplicationCode         string   `json:"application_code"`
	Issuer                  string   `json:"issuer"`
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret"`
	RedirectURIs            []string `json:"redirect_uris"`
	Scopes                  []string `json:"scopes"`
	ProvisioningEndpoint    string   `json:"provisioning_endpoint"`
	ProvisioningSecret      string   `json:"provisioning_secret"`
	ProvisioningKeyVersion  int64    `json:"provisioning_key_version"`
	ProvisioningFingerprint string   `json:"provisioning_fingerprint"`
	IssuedAt                string   `json:"issued_at"`
}

type Store struct {
	cache  cache.Cache
	cipher *appcrypto.EnvelopeCipher
	ttl    time.Duration
}

func New(c cache.Cache, cipher *appcrypto.EnvelopeCipher) (*Store, error) {
	if c == nil || c.Provider() == "disabled" || cipher == nil {
		return nil, ErrUnavailable
	}
	return &Store{cache: c, cipher: cipher, ttl: defaultTTL}, nil
}

func (s *Store) Issue(ctx context.Context, bundle Bundle) (string, time.Time, error) {
	if s == nil || s.cache == nil || s.cipher == nil || strings.TrimSpace(bundle.ApplicationCode) == "" || strings.TrimSpace(bundle.ClientSecret) == "" || strings.TrimSpace(bundle.ProvisioningSecret) == "" {
		return "", time.Time{}, ErrUnavailable
	}
	bundle.IssuedAt = time.Now().UTC().Format(time.RFC3339Nano)
	// #nosec G117 -- this purpose-built one-time credential bundle is encrypted
	// before cache storage and can only be consumed once within the short TTL.
	raw, err := json.Marshal(bundle)
	if err != nil {
		return "", time.Time{}, err
	}
	ciphertext, err := s.cipher.Encrypt(raw, []byte(aadValue))
	if err != nil {
		return "", time.Time{}, err
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	key := tokenKey(token)
	ok, err := s.cache.SetNX(ctx, key, ciphertext, s.ttl)
	if err != nil || !ok {
		if err == nil {
			err = errors.New("enrollment token collision")
		}
		return "", time.Time{}, err
	}
	return token, time.Now().UTC().Add(s.ttl), nil
}

func (s *Store) Consume(ctx context.Context, token string) (Bundle, error) {
	if s == nil || s.cache == nil || s.cipher == nil || len(strings.TrimSpace(token)) < 32 {
		return Bundle{}, ErrInvalidToken
	}
	key := tokenKey(token)
	value, err := s.cache.Get(ctx, key)
	if err != nil {
		return Bundle{}, ErrInvalidToken
	}
	consumed, err := s.cache.CompareAndDelete(ctx, key, value)
	if err != nil || !consumed {
		return Bundle{}, ErrInvalidToken
	}
	raw, err := s.cipher.Decrypt(value, []byte(aadValue))
	if err != nil {
		return Bundle{}, ErrInvalidToken
	}
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil || bundle.ApplicationCode == "" || bundle.ClientSecret == "" || bundle.ProvisioningSecret == "" {
		return Bundle{}, ErrInvalidToken
	}
	return bundle, nil
}

func tokenKey(token string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return "credential-handoff:" + hex.EncodeToString(digest[:])
}
