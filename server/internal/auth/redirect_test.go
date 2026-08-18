package auth

import "testing"

func TestSanitizeRedirect(t *testing.T) {
	h := &Handler{defaultRedirect: "/home"}

	cases := []struct {
		name     string
		in       string
		expected string
	}{
		{"空串回退", "", "/home"},
		{"正常站内路径", "/applications?keyword=dev", "/applications?keyword=dev"},
		{"根路径", "/", "/"},
		{"外部 URL 拒绝", "https://evil.com", "/home"},
		{"protocol-relative 拒绝", "//evil.com", "/home"},
		{"含 scheme 拒绝", "http:///path", "/home"},
		{"反斜杠绕过拒绝", `/\evil.com`, "/home"},
		{"反斜杠 scheme 拒绝", `https:\evil.com`, "/home"},
		{"百分号编码 // 拒绝", "/%2f%2fevil.com", "/home"},
		{"百分号编码反斜杠拒绝", "/%5cevil.com", "/home"},
		{"大写编码拒绝", "/%2F%5Cevil.com", "/home"},
		{"冒号拒绝", "/foo:bar", "/home"},
		{"js scheme 拒绝", "javascript:alert(1)", "/home"},
		{"空格修剪", "  /applications  ", "/applications"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := h.sanitizeRedirect(tc.in); got != tc.expected {
				t.Errorf("sanitizeRedirect(%q) = %q, want %q", tc.in, got, tc.expected)
			}
		})
	}
}
