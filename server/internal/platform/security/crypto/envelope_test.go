package crypto

import (
	"strings"
	"testing"
)

func TestEnvelopeCipherRoundTripAndAADBinding(t *testing.T) {
	provider, err := New("standard", strings.Repeat("k", 32), "v1")
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := NewEnvelopeCipher(provider)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := cipher.Encrypt([]byte("banking payload"), []byte("tenant:org-1"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := cipher.Decrypt(encoded, []byte("tenant:org-1"))
	if err != nil || string(plain) != "banking payload" {
		t.Fatalf("Decrypt() = %q, %v", plain, err)
	}
	if _, err := cipher.Decrypt(encoded, []byte("tenant:org-2")); err == nil {
		t.Fatal("ciphertext decrypted with a different AAD")
	}
}

func TestEnvelopeCipherRejectsMalformedPayload(t *testing.T) {
	provider, err := New("standard", strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := NewEnvelopeCipher(provider)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "v0.value", "env1.not-base64"} {
		if _, err := cipher.Decrypt(value, nil); err == nil {
			t.Fatalf("Decrypt(%q) accepted malformed payload", value)
		}
	}
}
