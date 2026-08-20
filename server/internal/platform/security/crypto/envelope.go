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
)

const envelopeVersion = "env1"

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
	ciphertext, err := sealDataKey(dataKey, plaintext, aad)
	if err != nil {
		return "", err
	}
	wrappedKey, err := e.provider.Encrypt(dataKey, aad)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(envelopePayload{WrappedKey: wrappedKey, Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext)})
	if err != nil {
		return "", err
	}
	return envelopeVersion + "." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func (e *EnvelopeCipher) Decrypt(encoded string, aad []byte) ([]byte, error) {
	parts := strings.SplitN(encoded, ".", 2)
	if len(parts) != 2 || parts[0] != envelopeVersion || parts[1] == "" {
		return nil, errors.New("invalid envelope ciphertext version")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var payload envelopePayload
	if err := json.Unmarshal(raw, &payload); err != nil || payload.WrappedKey == "" || payload.Ciphertext == "" {
		return nil, errors.New("invalid envelope ciphertext payload")
	}
	dataKey, err := e.provider.Decrypt(payload.WrappedKey, aad)
	if err != nil {
		return nil, err
	}
	if len(dataKey) != 32 {
		return nil, errors.New("invalid envelope data key length")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return nil, err
	}
	return openDataKey(dataKey, ciphertext, aad)
}

func sealDataKey(dataKey, plaintext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(dataKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, aad), nil
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
	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], aad)
}
