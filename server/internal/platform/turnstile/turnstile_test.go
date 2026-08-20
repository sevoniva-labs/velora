package turnstile

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer 返回模拟 siteverify 的 HTTP 服务器。
func mockServer(t *testing.T, respond func(r *http.Request) (int, any)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, payload := respond(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerifySuccess(t *testing.T) {
	var gotSecret, gotToken string
	srv := mockServer(t, func(r *http.Request) (int, any) {
		_ = r.ParseForm()
		gotSecret, gotToken = r.PostForm.Get("secret"), r.PostForm.Get("response")
		return http.StatusOK, map[string]any{"success": true, "error-codes": []string{}}
	})
	v := NewVerifier("test-secret")
	v.endpoint = srv.URL

	ok, err := v.Verify(t.Context(), "widget-token", "1.2.3.4")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "test-secret", gotSecret)
	assert.Equal(t, "widget-token", gotToken)
}

func TestVerifyRejected(t *testing.T) {
	srv := mockServer(t, func(_ *http.Request) (int, any) {
		return http.StatusOK, map[string]any{"success": false, "error-codes": []string{"invalid-input-response"}}
	})
	v := NewVerifier("test-secret")
	v.endpoint = srv.URL

	ok, err := v.Verify(t.Context(), "bad-token", "")
	require.NoError(t, err, "siteverify 明确拒绝属于业务结果，不应报系统错误")
	assert.False(t, ok, "siteverify 拒绝时不应通过")
}

func TestVerifyMissingConfig(t *testing.T) {
	v := NewVerifier("") // 未配置
	ok, err := v.Verify(t.Context(), "token", "")
	assert.Error(t, err)
	assert.False(t, ok)
	assert.False(t, v.Enabled())
}

func TestVerifyEmptyToken(t *testing.T) {
	v := NewVerifier("secret")
	ok, err := v.Verify(t.Context(), "", "")
	assert.Error(t, err, "空 token 应报错")
	assert.False(t, ok)
}

func TestVerifyNetworkError(t *testing.T) {
	// 指向不可达端点：网络失败按拒绝处理。
	v := NewVerifier("secret")
	v.endpoint = "http://127.0.0.1:1/siteverify"
	ok, err := v.Verify(t.Context(), "token", "")
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestVerifyBadResponse(t *testing.T) {
	srv := mockServer(t, func(_ *http.Request) (int, any) {
		return http.StatusOK, "not-json"
	})
	v := NewVerifier("secret")
	v.endpoint = srv.URL
	ok, err := v.Verify(t.Context(), "token", "")
	assert.Error(t, err, "畸形响应应报错")
	assert.False(t, ok)
}
