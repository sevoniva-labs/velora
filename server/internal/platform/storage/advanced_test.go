package storage

import (
	"testing"
	"time"
)

func TestAdvancedStorageValidation(t *testing.T) {
	if err := validateMultipartKey(""); err == nil {
		t.Fatal("empty multipart key accepted")
	}
	for _, key := range []string{"../escape", "a/../b", `a\\b`, "/absolute", "a//b"} {
		if err := validateMultipartKey(key); err == nil {
			t.Fatalf("unsafe multipart key %q accepted", key)
		}
	}
	if err := validateMultipartPart(10001); err == nil {
		t.Fatal("invalid multipart part accepted")
	}
	if err := validatePresignTTL(16 * time.Minute); err == nil {
		t.Fatal("overlong presign ttl accepted")
	}
	if err := validatePresignTTL(5 * time.Minute); err != nil {
		t.Fatalf("valid presign ttl rejected: %v", err)
	}
}
