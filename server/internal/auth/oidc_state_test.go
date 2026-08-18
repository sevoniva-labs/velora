package auth

import (
	"strings"
	"testing"
	"time"
)

// OIDC state 是安全关键路径：测试编解码往返、篡改检测、过期。
func TestOIDCStateRoundTrip(t *testing.T) {
	m, err := NewOIDCManagerStateTest()
	if err != nil {
		t.Fatal(err)
	}
	state := &oidcState{
		Redirect: "/home",
		Verifier: "pkce-verifier-abc",
		Nonce:    "nonce-xyz",
		Expires:  time.Now().Add(time.Hour).Unix(),
	}
	token, err := m.encodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := m.decodeState(token)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Redirect != "/home" || decoded.Verifier != "pkce-verifier-abc" || decoded.Nonce != "nonce-xyz" {
		t.Errorf("state round trip mismatch: %+v", decoded)
	}
}

func TestOIDCStateTamperDetected(t *testing.T) {
	m, _ := NewOIDCManagerStateTest()
	state := &oidcState{Redirect: "/home", Verifier: "v", Nonce: "n", Expires: time.Now().Add(time.Hour).Unix()}
	token, _ := m.encodeState(state)

	parts := strings.Split(token, ".")
	// base64url 字符集为 [A-Za-z0-9_-]：翻转 payload 首字符几乎必然改变内容。
	first := parts[0]
	var flipped string
	if first[0] == 'A' {
		flipped = "B" + first[1:]
	} else {
		flipped = "A" + first[1:]
	}
	if _, err := m.decodeState(flipped + "." + parts[1]); err == nil {
		t.Fatal("篡改后的 state 应校验失败")
	}
}

func TestOIDCStateExpired(t *testing.T) {
	m, _ := NewOIDCManagerStateTest()
	state := &oidcState{Redirect: "/", Verifier: "v", Nonce: "n", Expires: time.Now().Add(-time.Minute).Unix()}
	token, _ := m.encodeState(state)
	if _, err := m.decodeState(token); err == nil {
		t.Fatal("过期 state 应校验失败")
	}
}

// NewOIDCManagerStateTest 仅用于测试 state 编解码（不连接真实 Provider）。
func NewOIDCManagerStateTest() (*OIDCManager, error) {
	return &OIDCManager{secret: []byte("test-secret-key-32-bytes-long!!"), stateTTL: 10 * time.Minute}, nil
}
