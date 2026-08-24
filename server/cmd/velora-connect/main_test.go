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
		body := `{"code":"000000","data":{"application_id":"app-1","application_code":"order-center","issuer":"https://auth.example.test","client_id":"client-1","client_secret":"0123456789abcdef0123456789abcdef","redirect_uris":["https://app.example.test/callback"],"scopes":["openid"],"provisioning_endpoint":"https://app.example.test/provisioning","provisioning_secret":"abcdef0123456789abcdef0123456789","provisioning_key_version":"1","provisioning_fingerprint":"fingerprint","directory_token":"vd_0123456789abcdef0123456789abcdef0123456789","directory_base_path":"/api/v1/integrations/applications/app-1/directory"}}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	dir := t.TempDir()
	if err := enroll([]string{"--portal", "https://home.example.test", "--output", dir}, strings.NewReader(strings.Repeat("t", 43)+"\n"), client); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"velora.env", "oidc-client-secret", "provisioning-secret", "directory-token"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %04o", name, info.Mode().Perm())
		}
	}
	config, err := os.ReadFile(filepath.Join(dir, "velora.env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "VELORA_DIRECTORY_BASE_URL='https://home.example.test/api/v1/integrations/applications/app-1/directory'") {
		t.Fatalf("directory URL missing from config: %s", config)
	}
	if err := doctor([]string{"--config", filepath.Join(dir, "velora.env")}); err != nil {
		t.Fatalf("doctor rejected generated bundle: %v", err)
	}
}

func TestDoctorRejectsWorldReadableDirectoryToken(t *testing.T) {
	dir := t.TempDir()
	for name, value := range map[string]string{"oidc-client-secret": strings.Repeat("c", 32), "provisioning-secret": strings.Repeat("p", 32), "directory-token": "vd_" + strings.Repeat("d", 43)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := "VELORA_APPLICATION_ID='app-1'\nVELORA_APPLICATION_CODE='orders'\nVELORA_OIDC_ISSUER='https://auth.example.test'\nVELORA_OIDC_CLIENT_ID='client'\nVELORA_OIDC_CLIENT_SECRET_FILE='" + filepath.Join(dir, "oidc-client-secret") + "'\nVELORA_PROVISIONING_SECRET_FILE='" + filepath.Join(dir, "provisioning-secret") + "'\nVELORA_DIRECTORY_BASE_URL='https://home.example.test/api/v1/integrations/applications/app-1/directory'\nVELORA_DIRECTORY_TOKEN_FILE='" + filepath.Join(dir, "directory-token") + "'\n"
	configPath := filepath.Join(dir, "velora.env")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "directory-token"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doctor([]string{"--config", configPath}); err == nil || !strings.Contains(err.Error(), "VELORA_DIRECTORY_TOKEN_FILE") {
		t.Fatalf("doctor error = %v", err)
	}
}
