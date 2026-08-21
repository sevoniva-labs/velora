package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/emmansun/gmsm/sm4"
)

// KeyVersion describes one key in a provider keyring. Only the active version
// is used for encryption; older versions remain decrypt-only during rotation.
type KeyVersion struct {
	Version     string
	Active      bool
	DecryptOnly bool
}

// NewKeyring creates a versioned provider. The raw key map is expected to be
// loaded from an approved secret/KMS adapter and is never returned by this API.
func NewKeyring(name string, rawKeys map[string]string, activeVersion string) (Provider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name != "standard" && name != "gm" {
		return nil, fmt.Errorf("unknown crypto provider %q", name)
	}
	activeVersion = strings.TrimSpace(activeVersion)
	if err := validateKeyVersion(activeVersion); err != nil {
		return nil, err
	}
	if len(rawKeys) == 0 {
		return nil, errors.New("crypto keyring must contain at least one key")
	}
	keys := make(map[string][]byte, len(rawKeys))
	for version, rawKey := range rawKeys {
		version = strings.TrimSpace(version)
		if err := validateKeyVersion(version); err != nil {
			return nil, err
		}
		key, err := normalizeKey(rawKey)
		if err != nil {
			return nil, fmt.Errorf("crypto key %q: %w", version, err)
		}
		keys[version] = key
	}
	if _, ok := keys[activeVersion]; !ok {
		return nil, fmt.Errorf("active crypto key version %q is not present in keyring", activeVersion)
	}
	return &keyring{name: name, active: activeVersion, keys: keys}, nil
}

func validateKeyVersion(version string) error {
	if version == "" || strings.ContainsAny(version, ".\r\n\t ") {
		return errors.New("crypto key version must be non-empty and contain no dots or whitespace")
	}
	return nil
}

type keyring struct {
	name   string
	active string
	keys   map[string][]byte
}

func (k *keyring) Name() string       { return k.name }
func (k *keyring) KeyVersion() string { return k.active }

// KeyVersions returns metadata only, never key material.
func (k *keyring) KeyVersions() []KeyVersion {
	versions := make([]string, 0, len(k.keys))
	for version := range k.keys {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	out := make([]KeyVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, KeyVersion{Version: version, Active: version == k.active, DecryptOnly: version != k.active})
	}
	return out
}

func (k *keyring) Hash(data []byte) []byte {
	if k.name == "gm" {
		return (&gm{}).Hash(data)
	}
	return (&standard{}).Hash(data)
}

func (k *keyring) Encrypt(plaintext, aad []byte) (string, error) {
	block, err := keyringBlock(k.name, k.keys[k.active])
	if err != nil {
		return "", err
	}
	return seal(k.active, block, plaintext, aad)
}

func (k *keyring) Decrypt(encoded string, aad []byte) ([]byte, error) {
	version := k.active
	if i := strings.IndexByte(encoded, '.'); i >= 0 {
		version = encoded[:i]
	}
	key, ok := k.keys[version]
	if !ok {
		return nil, fmt.Errorf("ciphertext key version %q is not present in keyring", version)
	}
	block, err := keyringBlock(k.name, key)
	if err != nil {
		return nil, err
	}
	return open(version, block, encoded, aad)
}

func keyringBlock(name string, key []byte) (cipher.Block, error) {
	if name == "gm" {
		if len(key) < 16 {
			return nil, errors.New("gm crypto key must contain at least 16 bytes")
		}
		return sm4.NewCipher(key[:16])
	}
	return aes.NewCipher(key)
}
