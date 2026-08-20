// Package httpserver 组装 Gin 路由、中间件与健康/指标端点。
package httpserver

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
)

// contextUserIDKey 用于日志中间件记录 userId。
const contextUserIDKey = "velora.log_user_id"

// RequestID 生成或透传 X-Request-Id。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-Id"))
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		response.SetRequestID(c, id)
		c.Writer.Header().Set("X-Request-Id", id)
		c.Next()
	}
}

// Logger 结构化请求日志 + 请求耗时直方图。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		metrics.Observe("velora_http_request_duration_milliseconds", float64(time.Since(start).Milliseconds()))
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).Milliseconds(),
			"requestId", response.RequestID(c),
			"clientIP", c.ClientIP(),
		}
		if uid, ok := c.Get(contextUserIDKey); ok && uid != "" {
			attrs = append(attrs, "userId", uid)
		}
		if len(c.Errors) > 0 {
			slog.Error("http request", append(attrs, "errors", c.Errors.String())...)
			return
		}
		if c.Writer.Status() >= 500 {
			slog.Error("http request", attrs...)
		} else if c.Writer.Status() >= 400 {
			slog.Warn("http request", attrs...)
		} else {
			slog.Info("http request", attrs...)
		}
	}
}

// Recovery 捕获 panic 并返回统一 500。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered", "panic", r, "requestId", response.RequestID(c))
				response.Error(c, errs.Internal("服务内部错误", nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// CORS 仅对配置中的来源输出跨域头（fail-closed）。
// 未配置任何来源时不输出任何 CORS 头，避免"任意 Origin + 凭据"的放开基线。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[strings.TrimRight(origin, "/")] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, X-Request-Id")
			c.Header("Access-Control-Max-Age", "600")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// SecurityHeaders 基础安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "0")
		// Turnstile 人机验证资源（登录页 widget）：脚本/iframe/样式来自 challenges.cloudflare.com。
		// 注意：script-src 保持严格（无 'unsafe-inline'），Turnstile 显式渲染为外部脚本，无需内联。
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline' https://challenges.cloudflare.com; script-src 'self' https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; font-src 'self' https://challenges.cloudflare.com; connect-src 'self'")
		c.Next()
	}
}

// Auth 解析会话 Cookie 并注入当前用户；解析失败返回 401。
func Auth(sessions *auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(auth.SessionCookieName)
		if err != nil || token == "" {
			response.Error(c, errs.Unauthorized(""))
			c.Abort()
			return
		}
		session, err := sessions.Decode(token)
		if err != nil {
			response.Error(c, errs.Unauthorized(""))
			c.Abort()
			return
		}
		user := session.ToCurrentUser()
		auth.SetCurrentUser(c, user)
		c.Set(contextUserIDKey, user.ID)
		c.Next()
	}
}

// CSRF 双提交校验：写请求必须携带与 velora_csrf Cookie 一致的 X-CSRF-Token。
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			c.Next()
			return
		}
		cookie, err := c.Cookie(auth.CSRFCookieName)
		if err != nil || cookie == "" {
			response.ErrorWith(c, 403, errs.CodeCSRFInvalid, "CSRF 校验失败：缺少 CSRF Cookie")
			c.Abort()
			return
		}
		header := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
		if header == "" || header != cookie {
			response.ErrorWith(c, 403, errs.CodeCSRFInvalid, "CSRF 校验失败")
			c.Abort()
			return
		}
		c.Next()
	}
}
