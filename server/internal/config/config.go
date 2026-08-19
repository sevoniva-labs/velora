// Package config 负责加载 Velora 服务端配置。
// 配置来源优先级：环境变量 > .env 文件 > 内置默认值。
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 为 Velora 服务端运行配置。
type Config struct {
	Port     string
	Env      string
	LogLevel string

	DatabaseURL    string
	DBMaxOpenConns int
	DBMaxIdleConns int

	CasdoorIssuer          string
	CasdoorClientID        string
	CasdoorClientSecret    string
	CasdoorRedirectURI     string
	CasdoorDefaultRedirect string
	// Casdoor 管理员凭据：仅用于服务端同步应用列表（读取管理 API），不用于用户认证。
	CasdoorAdminUsername string
	CasdoorAdminPassword string

	SessionSecret string
	SessionTTL    time.Duration
	CookieSecure  bool
	CookieDomain  string

	AdminRole string

	CORSAllowedOrigins []string
	TrustedProxies     []string

	HealthCheckTimeout time.Duration

	// 邮件模块：凭证加密密钥（base64 32 字节；开发环境缺省时由 SESSION_SECRET 派生）
	// 与定时补偿同步间隔（分钟，0 禁用；IDLE 实时推送属 Phase 2）。
	MailCredentialKey string
	MailSyncInterval  time.Duration
}

// Load 读取 .env（若存在）与环境变量，返回配置。
// 缺少必填项（SESSION_SECRET 等）时返回错误，避免带默认密钥运行。
func Load() (*Config, error) {
	// 仅当文件存在时加载；不覆盖已有环境变量。
	_ = godotenv.Load()

	cfg := &Config{
		Port:     getEnv("VELORA_PORT", "8080"),
		Env:      getEnv("VELORA_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DatabaseURL:    getEnv("DATABASE_URL", ""),
		DBMaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),

		CasdoorIssuer:          getEnv("CASDOOR_ISSUER", ""),
		CasdoorClientID:        getEnv("CASDOOR_CLIENT_ID", ""),
		CasdoorClientSecret:    getEnv("CASDOOR_CLIENT_SECRET", ""),
		CasdoorRedirectURI:     getEnv("CASDOOR_REDIRECT_URI", ""),
		CasdoorDefaultRedirect: getEnv("CASDOOR_DEFAULT_REDIRECT", "/home"),
		CasdoorAdminUsername:   getEnv("CASDOOR_ADMIN_USERNAME", ""),
		CasdoorAdminPassword:   getEnv("CASDOOR_ADMIN_PASSWORD", ""),

		SessionSecret: getEnv("SESSION_SECRET", ""),
		SessionTTL:    time.Duration(getEnvInt("SESSION_TTL_HOURS", 168)) * time.Hour,
		CookieSecure:  getEnvBool("COOKIE_SECURE", false),
		CookieDomain:  getEnv("COOKIE_DOMAIN", ""),

		AdminRole: getEnv("VELORA_ADMIN_ROLE", "velora_admin"),

		HealthCheckTimeout: time.Duration(getEnvInt("HEALTH_CHECK_TIMEOUT_SECONDS", 5)) * time.Second,

		MailCredentialKey: getEnv("MAIL_CREDENTIAL_KEY", ""),
		MailSyncInterval:  time.Duration(getEnvInt("MAIL_SYNC_INTERVAL_MINUTES", 10)) * time.Minute,
	}

	if origins := getEnv("CORS_ALLOWED_ORIGINS", ""); origins != "" {
		for _, o := range strings.Split(origins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, o)
			}
		}
	}

	// 可信反向代理网段（默认仅回环；经 nginx/网关部署时配置其网段以保留真实客户端 IP）。
	if proxies := getEnv("TRUSTED_PROXIES", "127.0.0.1,::1"); proxies != "" {
		for _, p := range strings.Split(proxies, ",") {
			if p = strings.TrimSpace(p); p != "" {
				cfg.TrustedProxies = append(cfg.TrustedProxies, p)
			}
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.SessionSecret) == "" {
		return fmt.Errorf("SESSION_SECRET 未设置：请生成至少 32 字节随机值（openssl rand -hex 32）")
	}
	if len(c.SessionSecret) < 32 {
		return fmt.Errorf("SESSION_SECRET 长度不足 32 字节")
	}
	if strings.TrimSpace(c.CasdoorIssuer) == "" || strings.TrimSpace(c.CasdoorClientID) == "" {
		return fmt.Errorf("CASDOOR_ISSUER / CASDOOR_CLIENT_ID 未设置")
	}
	// clientSecret 同时用作 OIDC state / 回调签名密钥：为空将导致签名可伪造，必须显式配置。
	if strings.TrimSpace(c.CasdoorClientSecret) == "" {
		return fmt.Errorf("CASDOOR_CLIENT_SECRET 未设置：请填写 Casdoor 中 Velora 应用的 Client Secret")
	}
	if strings.TrimSpace(c.CasdoorRedirectURI) == "" {
		return fmt.Errorf("CASDOOR_REDIRECT_URI 未设置：请配置 OIDC 回调地址")
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL 未设置")
	}
	// 生产环境密钥硬性要求：
	// MAIL_CREDENTIAL_KEY 必须独立配置（禁止从 SESSION_SECRET 派生，
	// 避免"单密钥泄露导致邮件凭证加密双保险同时失效"）。
	if c.Env == "production" {
		if strings.TrimSpace(c.MailCredentialKey) == "" {
			return fmt.Errorf("生产环境必须配置 MAIL_CREDENTIAL_KEY（base64 编码 32 字节独立密钥，勿用 SESSION_SECRET 派生）")
		}
		raw, err := base64.StdEncoding.DecodeString(c.MailCredentialKey)
		if err != nil || len(raw) != 32 {
			return fmt.Errorf("MAIL_CREDENTIAL_KEY 必须为 base64 编码的 32 字节密钥")
		}
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}
