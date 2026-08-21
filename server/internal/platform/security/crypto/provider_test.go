package crypto

import "testing"

func TestProvidersRoundTrip(t *testing.T) {
	for _, name := range []string{"standard", "gm"} {
		t.Run(name, func(t *testing.T) {
			p, err := New(name, "test-key-that-is-not-for-production")
			if err != nil {
				t.Fatal(err)
			}
			ciphertext, err := p.Encrypt([]byte("secret"), []byte("aad"))
			if err != nil {
				t.Fatal(err)
			}
			plaintext, err := p.Decrypt(ciphertext, []byte("aad"))
			if err != nil {
				t.Fatal(err)
			}
			if string(plaintext) != "secret" {
				t.Fatalf("got %q", plaintext)
			}
			if _, err := p.Decrypt(ciphertext, []byte("different")); err == nil {
				t.Fatal("expected AAD verification failure")
			}
		})
	}
}

func TestKeyringDecryptsPreviousVersionDuringRotation(t *testing.T) {
	oldProvider, err := NewKeyring("standard", map[string]string{"v1": "old-test-key-that-is-not-for-production"}, "v1")
	if err != nil {
		t.Fatal(err)
	}
	rotatedProvider, err := NewKeyring("standard", map[string]string{
		"v1": "old-test-key-that-is-not-for-production",
		"v2": "new-test-key-that-is-not-for-production",
	}, "v2")
	if err != nil {
		t.Fatal(err)
	}
	oldCiphertext, err := oldProvider.Encrypt([]byte("old secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := rotatedProvider.Decrypt(oldCiphertext, []byte("aad"))
	if err != nil || string(plaintext) != "old secret" {
		t.Fatalf("rotated decrypt = %q, %v", plaintext, err)
	}
	newCiphertext, err := rotatedProvider.Encrypt([]byte("new secret"), []byte("aad"))
	if err != nil {
		t.Fatal(err)
	}
	if rotatedProvider.KeyVersion() != "v2" || len(newCiphertext) < 3 || newCiphertext[:3] != "v2." {
		t.Fatalf("new ciphertext/version = %q/%q", newCiphertext, rotatedProvider.KeyVersion())
	}
	versioned, ok := rotatedProvider.(interface{ KeyVersions() []KeyVersion })
	if !ok {
		t.Fatal("keyring provider did not expose version metadata")
	}
	versions := versioned.KeyVersions()
	if len(versions) != 2 || !versions[1].Active || !versions[0].DecryptOnly {
		t.Fatalf("key versions = %+v", versions)
	}
}

func TestKeyringRejectsMissingActiveOrUnknownVersion(t *testing.T) {
	if _, err := NewKeyring("standard", map[string]string{"v1": "test-key-that-is-not-for-production"}, "v2"); err == nil {
		t.Fatal("missing active version was accepted")
	}
	provider, err := NewKeyring("standard", map[string]string{"v1": "test-key-that-is-not-for-production"}, "v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Decrypt("v9.invalid", nil); err == nil {
		t.Fatal("unknown ciphertext version was accepted")
	}
}
