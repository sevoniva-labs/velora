package mfa

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestTOTPGenerateAndValidate(t *testing.T) {
	provider := TOTP{}
	secret, provisioningURL, err := provider.Generate("Forge Test", "operator@example.test")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if secret == "" || provisioningURL == "" {
		t.Fatal("Generate() returned an empty secret or provisioning URL")
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}
	if !provider.Validate(code, secret) {
		t.Fatal("Validate() rejected a current code")
	}
	if provider.Validate("00000000", secret) {
		t.Fatal("Validate() accepted an invalid code")
	}
}
