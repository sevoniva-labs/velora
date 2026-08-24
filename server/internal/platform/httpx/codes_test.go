package httpx

import "testing"

func TestNumericCodeLoginChallengeUnavailable(t *testing.T) {
	if got := NumericCode("LOGIN_CHALLENGE_UNAVAILABLE"); got != "400015" {
		t.Fatalf("NumericCode(LOGIN_CHALLENGE_UNAVAILABLE) = %q, want 400015", got)
	}
}

func TestNumericCodeUnknownFallsBackToInternal(t *testing.T) {
	if got := NumericCode("NOT_REGISTERED"); got != "900099" {
		t.Fatalf("NumericCode(NOT_REGISTERED) = %q, want 900099", got)
	}
}
