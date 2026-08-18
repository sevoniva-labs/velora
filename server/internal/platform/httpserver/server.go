package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/application"
	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/casdoor"
	"github.com/sevoniva-labs/velora/server/internal/category"
	"github.com/sevoniva-labs/velora/server/internal/config"
	"github.com/sevoniva-labs/velora/server/internal/favorite"
	"github.com/sevoniva-labs/velora/server/internal/mail"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
	"github.com/sevoniva-labs/velora/server/internal/portal"
	"github.com/sevoniva-labs/velora/server/internal/tag"
	"github.com/sevoniva-labs/velora/server/internal/todo"
	"github.com/sevoniva-labs/velora/server/internal/visit"
)

// Deps 为组装路由所需的依赖。
type Deps struct {
	Cfg       *config.Config
	DB        *gorm.DB
	Sessions  *auth.SessionStore
	OIDC      *auth.OIDCManager
	Audit     *audit.Service
	AdminRole string
	Mail      *mail.Service // 邮件服务（nil 时不注册邮件路由）
}

// New 组装 Gin Engine 与全部路由。
func New(deps Deps) (*gin.Engine, error) {
	if deps.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// 仅信任配置的代理网段（默认回环），防止 X-Forwarded-For 伪造绕过限流 / 污染审计 IP。
	if err := r.SetTrustedProxies(deps.Cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("设置可信代理失败: %w", err)
	}
	r.Use(Recovery(), RequestID(), SecurityHeaders())
	r.Use(CORS(deps.Cfg.CORSAllowedOrigins))
	r.Use(Logger())
	r.Use(rateLimit(300, time.Minute))

	RegisterHealth(r, deps.DB)

	api := r.Group("/api/v1")

	// --- 公开端点 ---
	api.GET("/system/version", func(c *gin.Context) {
		response.OK(c, gin.H{"application": "velora", "version": "0.1.0"})
	})

	oidcHandler := auth.NewHandler(deps.OIDC, deps.Sessions, deps.AdminRole, deps.Cfg.CasdoorDefaultRedirect,
		func(c *gin.Context, userID string) {
			deps.Audit.Record(c, audit.Entry{Operator: userID, Action: audit.ActionLogin, Resource: "session"})
		},
		func(c *gin.Context, userID string) {
			deps.Audit.Record(c, audit.Entry{Operator: userID, Action: audit.ActionLogout, Resource: "session"})
		},
	)
	api.GET("/auth/oidc/login", rateLimit(30, time.Minute), oidcHandler.Login)
	api.GET("/auth/oidc/callback", rateLimit(60, time.Minute), oidcHandler.Callback)
	// 账号密码登录（Casdoor ROPC 代理）：更严限流防暴力破解（每 IP 每分钟 10 次）。
	api.POST("/auth/login", rateLimit(10, time.Minute), oidcHandler.LoginWithPassword)

	// --- 受保护端点（登录 + CSRF） ---
	secured := api.Group("")
	secured.Use(Auth(deps.Sessions), CSRF())

	oidcHandler.Register(secured) // /auth/logout, /me

	categoryService := category.NewService(deps.DB)
	category.NewHandler(categoryService, deps.Audit, deps.AdminRole).Register(secured)

	tagService := tag.NewService(deps.DB)
	tag.NewHandler(tagService, deps.Audit, deps.AdminRole).Register(secured)

	appService := application.NewService(deps.DB, deps.AdminRole, deps.Cfg.CasdoorIssuer, deps.Cfg.HealthCheckTimeout)
	appRepo := application.NewRepository(deps.DB)
	visitService := visit.NewService(deps.DB)
	var syncClient *casdoor.Client
	if deps.Cfg.CasdoorAdminUsername != "" && deps.Cfg.CasdoorAdminPassword != "" {
		syncClient = casdoor.NewClient(deps.Cfg.CasdoorIssuer, deps.Cfg.CasdoorClientID, deps.Cfg.CasdoorClientSecret, deps.Cfg.CasdoorAdminUsername, deps.Cfg.CasdoorAdminPassword)
	}
	application.NewHandler(appService, appRepo, visitService, deps.Audit, deps.AdminRole, syncClient).Register(secured)

	favService := favorite.NewService(deps.DB)
	favorite.NewHandler(favService, appService, appRepo, deps.Audit).Register(secured)

	todoService := todo.NewService(deps.DB)
	todo.NewHandler(todoService, deps.Audit, deps.AdminRole).Register(secured)

	// 邮件模块：独立领域，与 Todo 通过引用关联（source_system='mail'）。
	if deps.Mail != nil {
		mail.NewHandler(deps.Mail, deps.Audit).Register(secured)
	}

	portalService := portal.NewService(deps.DB)
	portalHandler := portal.NewHandler(portalService, deps.Audit, deps.AdminRole)
	// 门户展示配置（名称/公告/缩放）公开只读：登录页无需登录即可显示门户名称与公告。
	portalHandler.RegisterPublic(api)
	portalHandler.Register(secured)

	audit.NewHandler(deps.Audit, deps.AdminRole).Register(secured)

	return r, nil
}

// rateLimit 简易内存限流：窗口内每 IP 允许 n 次。
func rateLimit(n int, window time.Duration) gin.HandlerFunc {
	type entry struct {
		count int
		start time.Time
	}
	var mu sync.Mutex
	buckets := map[string]*entry{}
	go func() {
		ticker := time.NewTicker(window)
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for k, e := range buckets {
				if now.Sub(e.start) > window {
					delete(buckets, k)
				}
			}
			mu.Unlock()
		}
	}()
	return func(c *gin.Context) {
		key := c.ClientIP()
		mu.Lock()
		e, ok := buckets[key]
		if !ok || time.Since(e.start) > window {
			e = &entry{start: time.Now()}
			buckets[key] = e
		}
		e.count++
		over := e.count > n
		mu.Unlock()
		if over {
			response.ErrorWith(c, http.StatusTooManyRequests, errs.CodeRateLimited, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

var _ = slog.Info
