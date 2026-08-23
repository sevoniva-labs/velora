package provisioninghttp

import (
	"os"
	"path/filepath"
	"testing"

	appcrypto "github.com/sevoniva-labs/velora/server/internal/platform/security/crypto"
)

func TestApplicationCodeFromTopic(t *testing.T) {
	for _, value := range []string{"spectra", "order-center", "app_2"} {
		got, err := applicationCodeFromTopic(ProvisioningTopicPrefix + value)
		if err != nil || got != value {
			t.Fatalf("applicationCodeFromTopic(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"", "Order", "../secret", "app.other"} {
		if _, err := applicationCodeFromTopic(ProvisioningTopicPrefix + value); err == nil {
			t.Fatalf("applicationCodeFromTopic(%q) accepted invalid value", value)
		}
	}
}

func TestReadSecretReference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "provisioning-secret")
	if err := os.WriteFile(path, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := readSecretReference("file://"+path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "0123456789abcdef0123456789abcdef" {
		t.Fatal("secret reference returned unexpected contents")
	}
	for _, reference := range []string{path, "https://example.test/secret", "file://host/secret", "file:///tmp/secret?x=1"} {
		if _, err := readSecretReference(reference, nil, nil); err == nil {
			t.Fatalf("readSecretReference(%q) accepted an unsafe reference", reference)
		}
	}
}

func TestReadEncryptedSecretReferenceBindsAAD(t *testing.T) {
	provider, err := appcrypto.New("standard", "test-provisioning-key-material-32-bytes")
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := appcrypto.NewEnvelopeCipher(provider)
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("velora:provisioning:org-1:app-1")
	encoded, err := cipher.Encrypt([]byte("0123456789abcdef0123456789abcdef"), aad)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := readSecretReference("enc:"+encoded, cipher, aad)
	if err != nil || secret != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("read encrypted secret = %q, %v", secret, err)
	}
	if _, err := readSecretReference("enc:"+encoded, cipher, []byte("other-app")); err == nil {
		t.Fatal("encrypted secret was not bound to its organization and application")
	}
}
