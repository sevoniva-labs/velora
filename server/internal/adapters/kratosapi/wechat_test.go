package kratosapi

import "testing"

func TestSafeWeChatReturnPath(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"/home", "/home"},
		{"/login/oauth/authorize?client_id=spectra", "/login/oauth/authorize?client_id=spectra"},
		{"https://evil.example", "/"},
		{"//evil.example", "/"},
		{"/\\evil", "/"},
	} {
		if got := safeReturnPath(tc.input); got != tc.want {
			t.Errorf("safeReturnPath(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}
