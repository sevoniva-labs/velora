package application

import (
	"context"
	"strings"
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

func TestURLLaunchProvider(t *testing.T) {
	p := URLLaunchProvider{}
	user := &auth.CurrentUser{ID: "u-1"}

	tests := []struct {
		name    string
		app     *Application
		wantErr bool
	}{
		{"launch url", &Application{LaunchURL: "https://git.example.internal", HomeURL: "https://home.example.internal"}, false},
		{"fallback home url", &Application{LaunchURL: "", HomeURL: "https://home.example.internal"}, false},
		{"empty urls", &Application{}, true},
		{"javascript scheme rejected", &Application{LaunchURL: "javascript:alert(1)"}, true},
		{"file scheme rejected", &Application{LaunchURL: "file:///etc/passwd"}, true},
		{"no host rejected", &Application{LaunchURL: "https:///path"}, true},
		{"malformed rejected", &Application{LaunchURL: "://bad"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Launch(context.Background(), tt.app, user)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.URL != tt.app.LaunchURL && got.URL != tt.app.HomeURL {
				t.Errorf("URL = %q, want launch/home url", got.URL)
			}
			if got.Target != "_blank" {
				t.Errorf("target = %q, want _blank", got.Target)
			}
		})
	}
}

func TestOIDCLaunchProvider(t *testing.T) {
	p := OIDCLaunchProvider{issuer: "https://casdoor.example.internal"}
	user := &auth.CurrentUser{ID: "u-1", Username: "carson"}

	app := &Application{
		CasdoorClientID: "app-client-1",
		LaunchURL:       "https://app.example.internal/callback",
	}
	got, err := p.Launch(context.Background(), app, user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got.URL, "https://casdoor.example.internal/login/oauth/authorize?") {
		t.Errorf("URL 应以 Casdoor authorize 开头: %s", got.URL)
	}
	for _, param := range []string{"client_id=app-client-1", "redirect_uri=https%3A%2F%2Fapp.example.internal%2Fcallback", "response_type=code", "scope=openid+profile+email", "state=", "login_hint=carson"} {
		if !strings.Contains(got.URL, param) {
			t.Errorf("URL 缺少参数 %s: %s", param, got.URL)
		}
	}

	// 缺少 client_id 应报错。
	if _, err := p.Launch(context.Background(), &Application{LaunchURL: "https://x.example"}, user); err == nil {
		t.Error("缺少 CasdoorClientID 应报错")
	}
	// 回调地址必须为 http/https。
	if _, err := p.Launch(context.Background(), &Application{CasdoorClientID: "c", LaunchURL: "javascript:x"}, user); err == nil {
		t.Error("非法回调地址应报错")
	}
}

func TestLaunchRegistryUnsupportedType(t *testing.T) {
	r := NewLaunchRegistry("https://casdoor.example.internal")
	app := &Application{SSOType: SSOTypeSAML, LaunchURL: "https://x.example"}
	if _, err := r.Launch(context.Background(), app, &auth.CurrentUser{ID: "u"}); err == nil {
		t.Error("未实现类型应返回错误（不假装实现）")
	}
	if r.Provider(SSOTypeURL) == nil || r.Provider(SSOTypeOIDC) == nil {
		t.Error("URL / OIDC Provider 应已注册")
	}
}
