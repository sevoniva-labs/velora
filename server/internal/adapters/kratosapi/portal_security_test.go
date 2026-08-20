package kratosapi

import "testing"

func TestForwardAuthHostMatchesRegisteredApplication(t *testing.T) {
	tests := []struct {
		name   string
		target string
		host   string
		want   bool
	}{
		{name: "exact", target: "https://legacy.example.test/login", host: "legacy.example.test", want: true},
		{name: "case insensitive", target: "https://Legacy.Example.Test", host: "legacy.example.test", want: true},
		{name: "wrong host", target: "https://legacy.example.test", host: "evil.example.test", want: false},
		{name: "missing host", target: "https://legacy.example.test", host: "", want: false},
		{name: "forwarded chain", target: "https://legacy.example.test", host: "legacy.example.test, evil.example.test", want: false},
		{name: "plaintext target", target: "http://legacy.example.test", host: "legacy.example.test", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := forwardAuthHostMatches(test.target, test.host); got != test.want {
				t.Fatalf("forwardAuthHostMatches(%q, %q) = %v, want %v", test.target, test.host, got, test.want)
			}
		})
	}
}
