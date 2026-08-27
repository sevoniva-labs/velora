package httpx

import "testing"

func TestNumericCodeLoginChallengeUnavailable(t *testing.T) {
	if got := NumericCode("LOGIN_CHALLENGE_UNAVAILABLE"); got != "400015" {
		t.Fatalf("NumericCode(LOGIN_CHALLENGE_UNAVAILABLE) = %q, want 400015", got)
	}
}

func TestNumericCodeProfileAndWeChatContracts(t *testing.T) {
	tests := map[string]string{
		"INVALID_USER_PROFILE":      "100028",
		"WECHAT_DISABLED":           "200034",
		"WECHAT_LOGIN_FAILED":       "200035",
		"PROFILE_CONTACT_CONFLICT":  "300016",
		"PROFILE_VERSION_CONFLICT":  "300017",
		"WECHAT_STATUS_UNAVAILABLE": "400016",
		"WECHAT_UNLINK_FAILED":      "400017",
	}
	for symbol, want := range tests {
		if got := NumericCode(symbol); got != want {
			t.Fatalf("NumericCode(%s) = %q, want %q", symbol, got, want)
		}
	}
}

func TestNumericCodeUnknownFallsBackToInternal(t *testing.T) {
	if got := NumericCode("NOT_REGISTERED"); got != "900099" {
		t.Fatalf("NumericCode(NOT_REGISTERED) = %q, want 900099", got)
	}
}
