package config

import (
	"strings"
	"testing"
)

func TestWeChatLoginFailsClosedOnIncompleteConfiguration(t *testing.T) {
	cfg := Default()
	cfg.Security.AuthMode = "oidc"
	cfg.Security.CasdoorPasswordLoginEnabled = true
	cfg.Security.CasdoorApplication = "velora"
	cfg.Security.CasdoorOrganization = "built-in"
	cfg.Security.WeChatLoginEnabled = true
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "wechat_app_id") || !strings.Contains(err.Error(), "wechat_callback_url") {
		t.Fatalf("incomplete WeChat configuration accepted: %v", err)
	}
	for _, callback := range []string{"http://auth.example/_velora/wechat/callback", "https://auth.example/callback", "https://auth.example/_velora/wechat/callback?code=x"} {
		copy := cfg
		copy.Security.WeChatAppID = "wx-test"
		copy.Security.WeChatProvider = "wechat-open"
		copy.Security.WeChatCallbackURL = callback
		if err := copy.Validate(); err == nil || !strings.Contains(err.Error(), "wechat_callback_url") {
			t.Errorf("callback %q accepted: %v", callback, err)
		}
	}
}
