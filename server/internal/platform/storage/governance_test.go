package storage

import (
	"strings"
	"testing"
	"time"
)

func TestGovernedWriteValidation(t *testing.T) {
	if err := validateGovernedWrite("object", ObjectWriteOptions{SSES3: true, SSEKMSKeyID: "kms-key"}); err == nil {
		t.Fatal("mutually exclusive encryption modes accepted")
	}
	if err := validateGovernedWrite("object", ObjectWriteOptions{ChecksumSHA256: "invalid"}); err == nil {
		t.Fatal("invalid checksum accepted")
	}
	if err := validateGovernedWrite("object", ObjectWriteOptions{RetainUntil: time.Now().UTC().Add(time.Minute)}); err == nil {
		t.Fatal("retention without checksum accepted")
	}
	if err := validateGovernedWrite("object", ObjectWriteOptions{
		ChecksumSHA256: "47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=",
		RetainUntil:    time.Now().UTC().Add(-time.Minute),
	}); err == nil {
		t.Fatal("expired retention accepted")
	}
	if err := validateGovernedWrite("object", ObjectWriteOptions{ChecksumSHA256: strings.Repeat("A", 44)}); err == nil {
		t.Fatal("invalid base64 checksum accepted")
	}
}
