package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestEnrollWritesSecretFilesWithOwnerOnlyPermissions(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		body := `{"code":"000000","data":{"application_code":"order-center","issuer":"https://auth.example.test","client_id":"client-1","client_secret":"0123456789abcdef0123456789abcdef","redirect_uris":["https://app.example.test/callback"],"scopes":["openid"],"provisioning_endpoint":"https://app.example.test/provisioning","provisioning_secret":"abcdef0123456789abcdef0123456789","provisioning_key_version":1,"provisioning_fingerprint":"fingerprint"}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	dir := t.TempDir()
	if err := enroll([]string{"--portal", "https://home.example.test", "--output", dir}, strings.NewReader(strings.Repeat("t", 43)+"\n"), client); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"velora.env", "oidc-client-secret", "provisioning-secret"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %04o", name, info.Mode().Perm())
		}
	}
}
