package masking

import "strings"

func Mobile(v string) string {
	if len(v) < 7 {
		return Stars(v)
	}
	return v[:3] + "****" + v[len(v)-4:]
}
func IDCard(v string) string {
	if len(v) < 8 {
		return Stars(v)
	}
	return v[:4] + strings.Repeat("*", len(v)-8) + v[len(v)-4:]
}
func BankCard(v string) string {
	if len(v) < 8 {
		return Stars(v)
	}
	return v[:4] + strings.Repeat("*", len(v)-8) + v[len(v)-4:]
}
func Email(v string) string {
	at := strings.IndexByte(v, '@')
	if at <= 1 {
		return Stars(v)
	}
	return v[:1] + strings.Repeat("*", at-1) + v[at:]
}
func Name(v string) string {
	rs := []rune(v)
	if len(rs) <= 1 {
		return "*"
	}
	return string(rs[:1]) + strings.Repeat("*", len(rs)-1)
}
func Stars(v string) string {
	if v == "" {
		return ""
	}
	n := len([]rune(v))
	if n > 8 {
		n = 8
	}
	return strings.Repeat("*", n)
}
