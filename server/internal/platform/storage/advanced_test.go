package storage

import (
	"testing"
	"time"
)

func TestAdvancedStorageValidation(t *testing.T) {
	if err := validateMultipartKey(""); err == nil {
		t.Fatal("empty multipart key accepted")
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
