package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/sevoniva-labs/velora/server/internal/application"
	"github.com/sevoniva-labs/velora/server/internal/audit"
	"github.com/sevoniva-labs/velora/server/internal/auth"
	"github.com/sevoniva-labs/velora/server/internal/casdoor"
	"github.com/sevoniva-labs/velora/server/internal/category"
	"github.com/sevoniva-labs/velora/server/internal/config"
	"github.com/sevoniva-labs/velora/server/internal/favorite"
	"github.com/sevoniva-labs/velora/server/internal/mail"
	"github.com/sevoniva-labs/velora/server/internal/oidcprovider"
	"github.com/sevoniva-labs/velora/server/internal/platform/errs"
	"github.com/sevoniva-labs/velora/server/internal/platform/lockout"
	"github.com/sevoniva-labs/velora/server/internal/platform/ratelimit"
	"github.com/sevoniva-labs/velora/server/internal/platform/response"
	"github.com/sevoniva-labs/velora/server/internal/portal"
	"github.com/sevoniva-labs/velora/server/internal/tag"
	"github.com/sevoniva-labs/velora/server/internal/todo"
	"github.com/sevoniva-labs/velora/server/internal/usercenter"
	"github.com/sevoniva-labs/velora/server/internal/visit"
)

// Deps 为组装路由所需的依赖。
type Deps struct {
	Cfg          *config.Config
	DB           *gorm.DB
	Sessions     *auth.SessionStore
	OIDC         *auth.OIDCManager
	Audit        *audit.Service
	AdminRole    string
	Mail         *mail.Service // 邮件服务（nil 时不注册邮件路由）
	OIDCProvider *oidcprovider.Service
	Redis        *redis.Client // nil 时限流/锁定降级内存
	LoginLimiter ratelimit.Limiter
	LoginLockout *lockout.Manager
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

	// 全局限流：Redis 分布式（多实例统一）或内存降级（单实例）。
	// key 按真实客户端 IP；登录等敏感端点另行收紧（见下）。
	globalLimiter := deps.LoginLimiter
	if globalLimiter == nil {
		globalLimiter = ratelimit.New(deps.Redis, ratelimit.Config{Limit: 300, Window: time.Minute})
	}
	r.Use(limiterMiddleware(globalLimiter, 300, time.Minute))

	RegisterHealth(r, deps.DB)

	// --- OIDC Provider（Phase B）：第三方应用 SSO 终点（公开，无需 Velora 登录） ---
	if deps.OIDCProvider != nil {
		deps.OIDCProvider.SetIssuer(deps.Cfg.PublicBaseURL)
		deps.OIDCProvider.SetUserSnapshot(func(ctx context.Context, userID string) (*auth.CurrentUser, error) {
			// 从服务端会话表读取用户最新快照（Phase B6）：
			// 保证 OIDC access_token 中的 preferred_username / roles 与最近登录一致。
			if deps.Sessions.DB() == nil {
				return &auth.CurrentUser{ID: userID}, nil
			}
			var rec auth.ServerSession
			err := deps.Sessions.DB().WithContext(ctx).
				Where("user_id = ? AND revoked_at IS NULL", userID).
				Order("last_active_at DESC").First(&rec).Error
			if err != nil {
				return &auth.CurrentUser{ID: userID}, nil
			}
			return rec.ToCurrentUser(), nil
		})
		oidcHandler := oidcprovider.NewHandler(
			deps.OIDCProvider,
			func(c *gin.Context) *auth.CurrentUser {
				// 从 Velora 会话 Cookie 解析当前用户（已登录返回用户，未登录返回 nil）
				token, err := c.Cookie(auth.SessionCookieName)
				if err != nil || token == "" {
					return nil
				}
				session, err := deps.Sessions.Decode(token)
				if err != nil || session.ExpiresAt.Before(time.Now()) {
					return nil
				}
				return session.ToCurrentUser()
			},
			func(c *gin.Context, authorizePath string) string {
				// 未登录：跳 Velora 登录页，登录后回 authorize
				return "/login?redirect=" + url.QueryEscape(authorizePath)
			},
			func(c *gin.Context, action, resource, detail string) {
				deps.Audit.Record(c, audit.Entry{Operator: "oidc", Action: action, Resource: resource, Detail: detail})
			},
		)
		oidcHandler.Register(r.Group("/oidc"))
	}

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
		func(c *gin.Context, username string) {
			deps.Audit.Record(c, audit.Entry{Operator: username, Action: audit.ActionLoginFailed, Resource: "session", Detail: "登录失败"})
		},
		deps.LoginLockout,
	)
	api.GET("/auth/oidc/login", limiterMiddleware(globalLimiter, 30, time.Minute), oidcHandler.Login)
	api.GET("/auth/oidc/callback", limiterMiddleware(globalLimiter, 60, time.Minute), oidcHandler.Callback)
	// 账号密码登录（Casdoor ROPC 代理）：更严限流防暴力破解（每 IP 每分钟 10 次）。
	api.POST("/auth/login", limiterMiddleware(globalLimiter, 10, time.Minute), oidcHandler.LoginWithPassword)

	// --- 受保护端点（登录 + CSRF） ---
	secured := api.Group("")
	secured.Use(Auth(deps.Sessions), CSRF())

	oidcHandler.Register(secured) // /auth/logout, /me

	categoryService := category.NewService(deps.DB)
	category.NewHandler(categoryService, deps.Audit, deps.AdminRole).Register(secured)

	tagService := tag.NewService(deps.DB)
	tag.NewHandler(tagService, deps.Audit, deps.AdminRole).Register(secured)

	appService := application.NewService(deps.DB, deps.AdminRole, deps.Cfg.CasdoorIssuer, deps.Cfg.PublicBaseURL, deps.Cfg.HealthCheckTimeout,
		func(ctx context.Context, applicationID uint64) (string, []string, bool, error) {
			// VELORA_OIDC 启动：按应用 ID 查 Velora OIDC client
			if deps.OIDCProvider == nil {
				return "", nil, false, nil
			}
			clients, err := deps.OIDCProvider.ListClientsByApplication(ctx, applicationID)
			if err != nil || len(clients) == 0 {
				return "", nil, false, err
			}
			return clients[0].ClientID, clients[0].RedirectURIs(), true, nil
		},
		func(ctx context.Context, applicationID uint64, redirectURIs []string) (string, string, error) {
			// VELORA_OIDC 应用创建时自动生成 client
			if deps.OIDCProvider == nil {
				return "", "", nil
			}
			client, secret, err := deps.OIDCProvider.CreateClient(ctx, applicationID, redirectURIs, nil)
			if err != nil {
				return "", "", err
			}
			return client.ClientID, secret, nil
		},
	)
	appRepo := application.NewRepository(deps.DB)
	visitService := visit.NewService(deps.DB)
	var syncClient *casdoor.Client
	if deps.Cfg.CasdoorAdminUsername != "" && deps.Cfg.CasdoorAdminPassword != "" {
		syncClient = casdoor.NewClient(deps.Cfg.CasdoorIssuer, deps.Cfg.CasdoorClientID, deps.Cfg.CasdoorClientSecret, deps.Cfg.CasdoorAdminUsername, deps.Cfg.CasdoorAdminPassword)
	}
	application.NewHandler(appService, appRepo, visitService, deps.Audit, deps.AdminRole, syncClient, deps.OIDCProvider).Register(secured)

	// 自助用户中心（Phase C4）：改密 / 个人资料（设备管理复用 /auth/sessions）
	if syncClient != nil {
		usercenter.NewHandler(syncClient, deps.Sessions, deps.AdminRole).Register(secured)
	}

	favService := favorite.NewService(deps.DB)
	favorite.NewHandler(favService, appService, appRepo, deps.Audit).Register(secured)

	todoService := todo.NewService(deps.DB)
	todo.NewHandler(todoService, deps.Audit, deps.AdminRole).Register(secured)

	// 邮件模块：独立领域，与 Todo 通过引用关联（source_system='mail'）。
	if deps.Mail != nil {
		mail.NewHandler(deps.Mail, deps.Audit).Register(secured)
	}

	portalService := portal.NewService(deps.DB)
	// 应用服务读取门户设置（「新」应用标识天数规则）。
	appService.SetSettingsReader(portalService)
	portalHandler := portal.NewHandler(portalService, deps.Audit, deps.AdminRole)
	// 门户展示配置（名称/公告/缩放）公开只读：登录页无需登录即可显示门户名称与公告。
	portalHandler.RegisterPublic(api)
	portalHandler.Register(secured)

	audit.NewHandler(deps.Audit, deps.AdminRole).Register(secured)

	return r, nil
}

// limiterMiddleware 用限流器实现中间件：每客户端 IP 在窗口内允许 n 次。
// 复用同一个 Limiter（Redis 连接/内存桶），通过带限流配置的 key 前缀区分不同阈值。
func limiterMiddleware(l ratelimit.Limiter, n int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("%d-%d:%s", n, window/time.Minute, c.ClientIP())
		allowed, _, err := l.Allow(c.Request.Context(), key)
		if err != nil {
			// 限流器故障：放行（fail-open），不因限流组件故障拖垮业务。
			c.Next()
			return
		}
		if !allowed {
			response.ErrorWith(c, http.StatusTooManyRequests, errs.CodeRateLimited, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}

var _ = slog.Info
