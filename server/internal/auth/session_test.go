package auth

import (
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *SessionStore {
	t.Helper()
	s, err := NewSessionStore(strings.Repeat("s", 32), time.Hour, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSessionEncodeDecodeRoundTrip(t *testing.T) {
	s := newTestStore(t)
	sess := s.NewSession(&CurrentUser{
		ID: "u-1", Username: "carson", DisplayName: "Carson",
		Email: "c@example.com", Organization: "sevoniva",
		Roles: []string{"velora_admin"}, Groups: []string{"g1"},
	})
	token, err := s.Encode(sess)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := s.Decode(token)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.UserID != "u-1" || decoded.Username != "carson" || decoded.DisplayName != "Carson" {
		t.Errorf("round trip mismatch: %+v", decoded)
	}
	if len(decoded.Roles) != 1 || decoded.Roles[0] != "velora_admin" {
		t.Errorf("roles mismatch: %+v", decoded.Roles)
	}
}

func TestSessionTamperDetected(t *testing.T) {
	s := newTestStore(t)
	sess := s.NewSession(&CurrentUser{ID: "u-1", Username: "a"})
	token, _ := s.Encode(sess)

	// 篡改 payload（把用户名改掉）。
	parts := strings.Split(token, ".")
	tampered := strings.ReplaceAll(parts[0], "a", "b") + "." + parts[1]
	if _, err := s.Decode(tampered); err == nil {
		t.Fatal("篡改后的会话应校验失败")
	}
}

func TestSessionExpired(t *testing.T) {
	store, err := NewSessionStore(strings.Repeat("x", 32), time.Second, false, "")
	if err != nil {
		t.Fatal(err)
	}
	sess := store.NewSession(&CurrentUser{ID: "u-1", Username: "a"})
	token, _ := store.Encode(sess)
	time.Sleep(1100 * time.Millisecond)
	if _, err := store.Decode(token); err == nil {
		t.Fatal("过期会话应校验失败")
	}
}

func TestSessionWrongSecret(t *testing.T) {
	s1, _ := NewSessionStore(strings.Repeat("a", 32), time.Hour, false, "")
	s2, _ := NewSessionStore(strings.Repeat("b", 32), time.Hour, false, "")
	sess := s1.NewSession(&CurrentUser{ID: "u-1"})
	token, _ := s1.Encode(sess)
	if _, err := s2.Decode(token); err == nil {
		t.Fatal("不同密钥应校验失败")
	}
}

func TestNewSessionStoreShortSecret(t *testing.T) {
	if _, err := NewSessionStore("short", time.Hour, false, ""); err == nil {
		t.Fatal("过短密钥应报错")
	}
}

func TestToCurrentUserCopy(t *testing.T) {
	s := newTestStore(t)
	sess := s.NewSession(&CurrentUser{ID: "u-1", Roles: []string{"r1"}})
	u := sess.ToCurrentUser()
	u.Roles[0] = "mutated"
	if sess.Roles[0] != "r1" {
		t.Fatal("ToCurrentUser 应复制切片，避免共享底层数组")
	}
}

func TestIsAdmin(t *testing.T) {
	u := &CurrentUser{Roles: []string{"developer"}}
	if u.IsAdmin("velora_admin") {
		t.Fatal("不应判定为管理员")
	}
	u.Roles = append(u.Roles, "velora_admin")
	if !u.IsAdmin("velora_admin") {
		t.Fatal("应判定为管理员")
	}
}
