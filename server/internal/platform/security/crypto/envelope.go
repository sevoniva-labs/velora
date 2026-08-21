package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const (
	envelopeVersion       = "env2"
	legacyEnvelopeVersion = "env1"
)

// EnvelopeCipher uses the configured Provider as a KEK boundary and a fresh
// random AES-256 data key for every payload. The provider therefore controls
// key versioning while payload encryption remains independent of KMS/HSM SDKs.
type EnvelopeCipher struct {
	provider Provider
}

type envelopePayload struct {
	WrappedKey string `json:"k"`
	Ciphertext string `json:"c"`
}

// Envelope is the versioned, auditable envelope format exchanged between
// business code and a CryptoProvider. Key material never appears in it.
type Envelope struct {
	Version    string    `json:"version"`
	Provider   string    `json:"provider"`
	Algorithm  string    `json:"algorithm"`
	KeyID      string    `json:"keyId"`
	Nonce      string    `json:"nonce"`
	Ciphertext string    `json:"ciphertext"`
	WrappedKey string    `json:"wrappedKey"`
	CreatedAt  time.Time `json:"createdAt"`
}

func NewEnvelopeCipher(provider Provider) (*EnvelopeCipher, error) {
	if provider == nil {
		return nil, errors.New("envelope crypto provider is required")
	}
	return &EnvelopeCipher{provider: provider}, nil
}

func (e *EnvelopeCipher) Encrypt(plaintext, aad []byte) (string, error) {
	dataKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dataKey); err != nil {
		return "", err
	}
	nonce, ciphertext, err := sealDataKey(dataKey, plaintext, aad)
	if err != nil {
		return "", err
	}
	wrappedKey, err := e.provider.Encrypt(dataKey, aad)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(Envelope{
		Version: envelopeVersion, Provider: e.provider.Name(), Algorithm: "AES-256-GCM", KeyID: e.provider.KeyVersion(),
		Nonce: base64.RawURLEncoding.EncodeToString(nonce), Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
		WrappedKey: wrappedKey, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}
	return envelopeVersion + "." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (e *EnvelopeCipher) Decrypt(encoded string, aad []byte) ([]byte, error) {
	parts := strings.SplitN(encoded, ".", 2)
	if len(parts) != 2 || parts[1] == "" {
		return nil, errors.New("invalid envelope ciphertext version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	if parts[0] == legacyEnvelopeVersion {
		var legacy envelopePayload
		if err := json.Unmarshal(raw, &legacy); err != nil || legacy.WrappedKey == "" || legacy.Ciphertext == "" {
			return nil, errors.New("invalid envelope ciphertext payload")
		}
		dataKey, err := e.provider.Decrypt(legacy.WrappedKey, aad)
		if err != nil {
			return nil, err
		}
		if len(dataKey) != 32 {
			return nil, errors.New("invalid envelope data key length")
		}
		ciphertext, err := base64.RawURLEncoding.DecodeString(legacy.Ciphertext)
		if err != nil {
			return nil, err
		}
		return openDataKey(dataKey, ciphertext, aad)
	}
	if parts[0] != envelopeVersion {
		return nil, errors.New("invalid envelope ciphertext version")
	}
	var payload Envelope
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Version != envelopeVersion || payload.Provider == "" || payload.Algorithm != "AES-256-GCM" || payload.KeyID == "" || payload.Nonce == "" || payload.WrappedKey == "" || payload.Ciphertext == "" || payload.CreatedAt.IsZero() {
		return nil, errors.New("invalid envelope ciphertext payload")
	}
	if payload.Provider != e.provider.Name() || payload.KeyID != e.provider.KeyVersion() {
		return nil, errors.New("envelope provider or key version is not active")
	}
	dataKey, err := e.provider.Decrypt(payload.WrappedKey, aad)
	if err != nil {
		return nil, err
	}
	if len(dataKey) != 32 {
		return nil, errors.New("invalid envelope data key length")
	}
	nonce, err := base64.RawURLEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return nil, err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, err
	}
	return openDataKeyWithNonce(dataKey, nonce, ciphertext, aad)
}

func sealDataKey(dataKey, plaintext, aad []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return nonce, gcm.Seal(nil, nonce, plaintext, aad), nil
}

func openDataKey(dataKey, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("envelope ciphertext too short")
	}
	return openDataKeyWithNonce(dataKey, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], aad)
}

func openDataKeyWithNonce(dataKey, nonce, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("envelope nonce has invalid length")
	}
	return gcm.Open(nil, nonce, ciphertext, aad)
}
