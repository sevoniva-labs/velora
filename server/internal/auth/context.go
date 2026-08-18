package auth

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
)

// ctxCurrentUserKey 为 gin context 中当前用户键。
const ctxCurrentUserKey = "velora.current_user"

// SetCurrentUser 把当前用户写入请求上下文。
func SetCurrentUser(c *gin.Context, u *CurrentUser) {
	c.Set(ctxCurrentUserKey, u)
}

// CurrentUserFrom 从请求上下文读取当前用户；未登录返回 nil。
func CurrentUserFrom(c *gin.Context) *CurrentUser {
	v, ok := c.Get(ctxCurrentUserKey)
	if !ok {
		return nil
	}
	u, ok := v.(*CurrentUser)
	if !ok {
		return nil
	}
	return u
}

// RequireUser 返回当前用户，未登录返回错误。
func RequireUser(c *gin.Context) (*CurrentUser, error) {
	u := CurrentUserFrom(c)
	if u == nil {
		return nil, errors.New("unauthorized")
	}
	return u, nil
}

// UserIDFrom 返回当前用户 ID（审计等场景使用），未登录返回空串。
func UserIDFrom(c *gin.Context) string {
	if u := CurrentUserFrom(c); u != nil {
		return u.ID
	}
	return ""
}

// contextUser 便于在非 gin 场景传递用户。
type contextUserKey struct{}

// WithUser 把用户放入 context.Context。
func WithUser(ctx context.Context, u *CurrentUser) context.Context {
	return context.WithValue(ctx, contextUserKey{}, u)
}

// UserFromContext 从 context.Context 读取用户。
func UserFromContext(ctx context.Context) *CurrentUser {
	u, _ := ctx.Value(contextUserKey{}).(*CurrentUser)
	return u
}
