// Package permission 提供访问控制中间件与判定工具。
package permission

import (
	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// AdminRequired 校验当前用户持有 Velora 管理员角色；否则 403。
func AdminRequired(adminRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := auth.RequireUser(c)
		if err != nil {
			response.Error(c, errs.Unauthorized(""))
			c.Abort()
			return
		}
		if !user.IsAdmin(adminRole) {
			response.Error(c, errs.Forbidden("需要管理员权限"))
			c.Abort()
			return
		}
		c.Next()
	}
}
