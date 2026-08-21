package password

import "testing"

func TestHasherRoundTrip(t *testing.T) {
	h := DefaultHasher()
	encoded, err := h.Hash("StrongPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if !h.Verify("StrongPassword123!", encoded) {
		t.Fatal("expected password to verify")
	}
	if h.Verify("wrong-password", encoded) {
		t.Fatal("wrong password verified")
	}
}

func TestPolicy(t *testing.T) {
	p := Policy{
		MinLength:     12,
		RequireUpper:  true,
		RequireLower:  true,
		RequireDigit:  true,
		RequireSymbol: true,
	}
	if err := p.Validate("Short1!"); err == nil {
		t.Fatal("expected short password rejection")
	}
	if err := p.Validate("alllowercasepassword"); err == nil {
		t.Fatal("expected character-class rejection")
	}
	if err := p.Validate("GoodPassword123!"); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
}
