package usercenter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/sevoniva-labs/velora/server/internal/auth"
)

// fakePasswordUpdater 模拟 Casdoor 客户端。
type fakePasswordUpdater struct {
	verifyErr error
	updateErr error
	updatedTo string
}

func (f *fakePasswordUpdater) VerifyPassword(_ context.Context, _, password string) error {
	return f.verifyErr
}

func (f *fakePasswordUpdater) UpdateUserPassword(_ context.Context, userID, newPassword string) error {
	f.updatedTo = newPassword
	return f.updateErr
}

func TestValidatePassword(t *testing.T) {
	assert.NotEmpty(t, validatePassword("short1"), "太短应拒绝")
	assert.NotEmpty(t, validatePassword("onlyletters"), "无数字应拒绝")
	assert.NotEmpty(t, validatePassword("12345678"), "无字母应拒绝")
	assert.Empty(t, validatePassword("StrongPass1"), "合法密码应通过")
	assert.Empty(t, validatePassword("密码with数字1"), "含数字与字母（中文算字母？）")
}

func TestChangePasswordOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakePasswordUpdater{}
	h := NewHandler(fake, nil, "velora_admin")

	r := gin.New()
	r.POST("/change-password", func(c *gin.Context) {
		auth.SetCurrentUser(c, &auth.CurrentUser{ID: "u-1", Username: "alice"})
		h.changePassword(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/change-password",
		strings.NewReader(`{"oldPassword":"OldPass1","newPassword":"NewPass1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "000000")
	assert.Equal(t, "NewPass1", fake.updatedTo)
}

func TestChangePasswordWrongOld(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &fakePasswordUpdater{verifyErr: errors.New("当前密码不正确")}
	h := NewHandler(fake, nil, "velora_admin")

	r := gin.New()
	r.POST("/change-password", func(c *gin.Context) {
		auth.SetCurrentUser(c, &auth.CurrentUser{ID: "u-1", Username: "alice"})
		h.changePassword(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/change-password",
		strings.NewReader(`{"oldPassword":"Wrong1","newPassword":"NewPass1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "", fake.updatedTo, "旧密码错误不应执行更新")
}
