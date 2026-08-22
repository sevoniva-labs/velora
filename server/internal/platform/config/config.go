package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/securefile"
	"gopkg.in/yaml.v3"
)

// Config is the complete runtime configuration. Secrets use yaml:"-" and are
// populated only from environment variables or *_FILE references.
type Config struct {
	App           App           `yaml:"app"`
	Server        Server        `yaml:"server"`
	Database      Database      `yaml:"database"`
	Cache         Cache         `yaml:"cache"`
	Messaging     Messaging     `yaml:"messaging"`
	Streaming     Streaming     `yaml:"streaming"`
	Search        Search        `yaml:"search"`
	Storage       Storage       `yaml:"storage"`
	Discovery     Discovery     `yaml:"discovery"`
	RemoteConfig  RemoteConfig  `yaml:"remote_config"`
	Security      Security      `yaml:"security"`
	Observability Observability `yaml:"observability"`
	Resilience    Resilience    `yaml:"resilience"`
	Compliance    Compliance    `yaml:"compliance"`
	Features      Features      `yaml:"features"`
	Provisioning  Provisioning  `yaml:"provisioning"`
}

type App struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
	Region      string `yaml:"region"`
	Zone        string `yaml:"zone"`
}

type Server struct {
	ListenAddr             string        `yaml:"listen_addr"`
	GRPCListenAddr         string        `yaml:"grpc_listen_addr"`
	PublicURL              string        `yaml:"public_url"`
	WebDir                 string        `yaml:"web_dir"`
	WebCSPFrameSources     []string      `yaml:"web_csp_frame_sources"`
	WebCSPConnectSources   []string      `yaml:"web_csp_connect_sources"`
	WebCSPWujieEnabled     bool          `yaml:"web_csp_wujie_enabled"`
	WebCSPWujieApprovalRef string        `yaml:"web_csp_wujie_approval_ref"`
	ReadTimeout            time.Duration `yaml:"read_timeout"`
	WriteTimeout           time.Duration `yaml:"write_timeout"`
	IdleTimeout            time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout        time.Duration `yaml:"shutdown_timeout"`
	MaxBodyBytes           int64         `yaml:"max_body_bytes"`
	TLSEnabled             bool          `yaml:"tls_enabled"`
	TLSCertFile            string        `yaml:"tls_cert_file"`
	TLSKeyFile             string        `yaml:"tls_key_file"`
	TLSClientCAFile        string        `yaml:"tls_client_ca_file"`
	RequireClientTLS       bool          `yaml:"require_client_tls"`
}

type Database struct {
	Provider     string        `yaml:"provider"` // postgres | mysql | oceanbase
	DSN          string        `yaml:"-"`
	MaxOpenConns int           `yaml:"max_open_conns"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
	MaxLifetime  time.Duration `yaml:"max_lifetime"`
	QueryTimeout time.Duration `yaml:"query_timeout"`
	AutoMigrate  bool          `yaml:"auto_migrate"`
	ReadOnlyDSN  string        `yaml:"-"`
}

type Cache struct {
	Provider      string        `yaml:"provider"` // disabled | memory | redis
	Mode          string        `yaml:"mode"`     // standalone | sentinel | cluster
	Addresses     []string      `yaml:"addresses"`
	MasterName    string        `yaml:"master_name"`
	Username      string        `yaml:"-"`
	Password      string        `yaml:"-"`
	DB            int           `yaml:"db"`
	Prefix        string        `yaml:"prefix"`
	TTL           time.Duration `yaml:"default_ttl"`
	TLS           bool          `yaml:"tls"`
	TLSCAFile     string        `yaml:"tls_ca_file"`
	TLSCertFile   string        `yaml:"tls_cert_file"`
	TLSKeyFile    string        `yaml:"tls_key_file"`
	TLSServerName string        `yaml:"tls_server_name"`
}

type Messaging struct {
	Provider string `yaml:"provider"` // disabled | rocketmq

	// RocketMQ 5 gRPC Proxy transport security.
	TLS           bool   `yaml:"tls"`
	TLSCAFile     string `yaml:"tls_ca_file"`
	TLSCertFile   string `yaml:"tls_cert_file"`
	TLSKeyFile    string `yaml:"tls_key_file"`
	TLSServerName string `yaml:"tls_server_name"`

	// RocketMQ 5.x uses Apache's official gRPC client and requires a Proxy
	// endpoint. Access credentials are environment/file-only secrets.
	RocketMQEndpoint          string        `yaml:"rocketmq_endpoint"`
	RocketMQGroup             string        `yaml:"rocketmq_group"`
	RocketMQNamespace         string        `yaml:"rocketmq_namespace"`
	RocketMQTopics            []string      `yaml:"rocketmq_topics"`
	RocketMQAccessKey         string        `yaml:"-"`
	RocketMQSecretKey         string        `yaml:"-"`
	RocketMQBatchSize         int           `yaml:"rocketmq_batch_size"`
	RocketMQInvisibleDuration time.Duration `yaml:"rocketmq_invisible_duration"`
	RocketMQAwaitDuration     time.Duration `yaml:"rocketmq_await_duration"`
}

type Streaming struct {
	Provider      string   `yaml:"provider"` // disabled | kafka
	Brokers       []string `yaml:"brokers"`
	ClientID      string   `yaml:"client_id"`
	Username      string   `yaml:"-"`
	Password      string   `yaml:"-"`
	TLS           bool     `yaml:"tls"`
	TLSCAFile     string   `yaml:"tls_ca_file"`
	TLSCertFile   string   `yaml:"tls_cert_file"`
	TLSKeyFile    string   `yaml:"tls_key_file"`
	TLSServerName string   `yaml:"tls_server_name"`
}

type Search struct {
	Provider      string   `yaml:"provider"` // disabled | elasticsearch | opensearch
	URLs          []string `yaml:"urls"`
	Username      string   `yaml:"-"`
	Password      string   `yaml:"-"`
	TLS           bool     `yaml:"tls"`
	TLSCAFile     string   `yaml:"tls_ca_file"`
	TLSCertFile   string   `yaml:"tls_cert_file"`
	TLSKeyFile    string   `yaml:"tls_key_file"`
	TLSServerName string   `yaml:"tls_server_name"`
}

type Storage struct {
	Provider      string `yaml:"provider"` // local | s3 | s3-compatible | minio | ceph-rgw | oss | cos | obs
	LocalRoot     string `yaml:"local_root"`
	Endpoint      string `yaml:"endpoint"`
	Region        string `yaml:"region"`
	Bucket        string `yaml:"bucket"`
	Prefix        string `yaml:"prefix"`
	AccessKey     string `yaml:"-"`
	SecretKey     string `yaml:"-"`
	SessionToken  string `yaml:"-"`
	PathStyle     bool   `yaml:"path_style"`
	TLS           bool   `yaml:"tls"`
	SSEMode       string `yaml:"sse_mode"` // none | s3 | kms
	SSEKMSKeyID   string `yaml:"sse_kms_key_id"`
	TLSCAFile     string `yaml:"tls_ca_file"`
	TLSCertFile   string `yaml:"tls_cert_file"`
	TLSKeyFile    string `yaml:"tls_key_file"`
	TLSServerName string `yaml:"tls_server_name"`
	// CapabilityContractFile points to immutable, target-specific S3 contract
	// evidence. It is intentionally environment/file-only and never belongs in
	// a shared YAML profile.
	CapabilityContractFile string `yaml:"-"`
}

type Discovery struct {
	Provider          string            `yaml:"provider"` // disabled | nacos
	Servers           []string          `yaml:"servers"`
	Namespace         string            `yaml:"namespace"`
	Group             string            `yaml:"group"`
	Cluster           string            `yaml:"cluster"`
	ServiceName       string            `yaml:"service_name"`
	GRPCServiceName   string            `yaml:"grpc_service_name"`
	AdvertiseIP       string            `yaml:"advertise_ip"`
	AdvertisePort     uint64            `yaml:"advertise_port"`
	AdvertiseGRPCPort uint64            `yaml:"advertise_grpc_port"`
	Weight            float64           `yaml:"weight"`
	Metadata          map[string]string `yaml:"metadata"`
	Username          string            `yaml:"-"`
	Password          string            `yaml:"-"`
	TLSRequired       bool              `yaml:"tls_required"`
	TLSCAFile         string            `yaml:"tls_ca_file"`
	TLSCertFile       string            `yaml:"tls_cert_file"`
	TLSKeyFile        string            `yaml:"tls_key_file"`
	TLSServerName     string            `yaml:"tls_server_name"`
}

type RemoteConfig struct {
	Provider      string   `yaml:"provider"` // disabled | nacos
	Servers       []string `yaml:"servers"`
	Namespace     string   `yaml:"namespace"`
	Group         string   `yaml:"group"`
	DataID        string   `yaml:"data_id"`
	FailFast      bool     `yaml:"fail_fast"`
	Username      string   `yaml:"-"`
	Password      string   `yaml:"-"`
	TLSRequired   bool     `yaml:"tls_required"`
	TLSCAFile     string   `yaml:"tls_ca_file"`
	TLSCertFile   string   `yaml:"tls_cert_file"`
	TLSKeyFile    string   `yaml:"tls_key_file"`
	TLSServerName string   `yaml:"tls_server_name"`
}

type Security struct {
	AuthMode                            string        `yaml:"auth_mode"` // oidc | password (development only)
	SessionTTL                          time.Duration `yaml:"session_ttl"`
	SecureCookies                       bool          `yaml:"secure_cookies"`
	SameSite                            string        `yaml:"same_site"`
	PasswordMinLength                   int           `yaml:"password_min_length"`
	PasswordUpper                       bool          `yaml:"password_require_upper"`
	PasswordLower                       bool          `yaml:"password_require_lower"`
	PasswordDigit                       bool          `yaml:"password_require_digit"`
	PasswordSymbol                      bool          `yaml:"password_require_symbol"`
	PasswordHistory                     int           `yaml:"password_history"`
	PasswordMaxAgeDay                   int           `yaml:"password_max_age_days"`
	LoginMaxFailures                    int           `yaml:"login_max_failures"`
	LoginLockDuration                   time.Duration `yaml:"login_lock_duration"`
	CryptoProvider                      string        `yaml:"crypto_provider"` // standard | gm
	CryptoAdapter                       string        `yaml:"crypto_adapter"`  // software | kms | hsm | pkcs11
	CryptoKey                           string        `yaml:"-"`
	CryptoKeyVersion                    string        `yaml:"crypto_key_version"`
	OIDCIssuer                          string        `yaml:"oidc_issuer"`
	OIDCName                            string        `yaml:"oidc_name"`
	OIDCClientID                        string        `yaml:"oidc_client_id"`
	OIDCClientSecret                    string        `yaml:"-"`
	OIDCRedirectURL                     string        `yaml:"oidc_redirect_url"`
	OIDCPostLogoutRedirectURL           string        `yaml:"oidc_post_logout_redirect_url"`
	CasdoorAccountURL                   string        `yaml:"casdoor_account_url"`
	CasdoorAdminURL                     string        `yaml:"casdoor_admin_url"`
	CasdoorAllowedHosts                 []string      `yaml:"casdoor_allowed_hosts"`
	ApplicationOnboardingV2             bool          `yaml:"application_onboarding_v2"`
	CasdoorAdminEntryEnabled            bool          `yaml:"casdoor_admin_entry_enabled"`
	CasdoorApplicationAutomationEnabled bool          `yaml:"casdoor_application_automation_enabled"`
	CasdoorAutomationToken              string        `yaml:"-"`
	CasdoorIdentityManagementEnabled    bool          `yaml:"casdoor_identity_management_enabled"`
	CasdoorIdentityClientID             string        `yaml:"casdoor_identity_client_id"`
	CasdoorIdentityClientSecret         string        `yaml:"-"`
	// CasdoorPasswordLoginEnabled keeps the browser login form in Velora while
	// delegating credential verification to Casdoor's application login API.
	// It is intentionally opt-in because this is a password-grant compatibility
	// mode, not the preferred Authorization Code + PKCE flow.
	CasdoorPasswordLoginEnabled bool   `yaml:"casdoor_password_login_enabled"`
	CasdoorApplication          string `yaml:"casdoor_application"`
	CasdoorOrganization         string `yaml:"casdoor_organization"`
	// Turnstile protects the Velora-hosted credential form. The secret is
	// environment/file-only; the site key and hostname allowlist are public
	// deployment configuration.
	TurnstileSiteKey    string   `yaml:"turnstile_site_key"`
	TurnstileSecret     string   `yaml:"-"`
	TurnstileHostnames  []string `yaml:"turnstile_hostnames"`
	TurnstileAction     string   `yaml:"turnstile_action"`
	OIDCInternalURL     string   `yaml:"oidc_internal_url"`
	OIDCProviderEnabled bool     `yaml:"oidc_provider_enabled"`
	AllowedOrigins      []string `yaml:"allowed_origins"`
	TrustedProxies      []string `yaml:"trusted_proxies"`
}

type Provisioning struct {
	SpectraEnabled bool   `yaml:"spectra_enabled"`
	SpectraURL     string `yaml:"spectra_url"`
	SpectraSecret  string `yaml:"-"`
}

func (s Security) TurnstileConfigured() bool {
	return strings.TrimSpace(s.TurnstileSiteKey) != "" && strings.TrimSpace(s.TurnstileSecret) != "" && len(s.TurnstileHostnames) > 0
}

func (s Security) EffectiveTurnstileAction() string {
	if action := strings.TrimSpace(s.TurnstileAction); action != "" {
		return action
	}
	return "login"
}

type Observability struct {
	LogLevel          string `yaml:"log_level"`
	LogFormat         string `yaml:"log_format"`
	MetricsEnabled    bool   `yaml:"metrics_enabled"`
	MetricsPath       string `yaml:"metrics_path"`
	TracingEnabled    bool   `yaml:"tracing_enabled"`
	OTLPEndpoint      string `yaml:"otlp_endpoint"`
	OTLPTLS           bool   `yaml:"otlp_tls"`
	OTLPTLSCAFile     string `yaml:"otlp_tls_ca_file"`
	OTLPTLSCertFile   string `yaml:"otlp_tls_cert_file"`
	OTLPTLSKeyFile    string `yaml:"otlp_tls_key_file"`
	OTLPTLSServerName string `yaml:"otlp_tls_server_name"`
	PprofEnabled      bool   `yaml:"pprof_enabled"`
}

type Resilience struct {
	DependencyTimeout       time.Duration `yaml:"dependency_timeout"`
	RetryMaxAttempts        int           `yaml:"retry_max_attempts"`
	RetryBaseDelay          time.Duration `yaml:"retry_base_delay"`
	CircuitFailureThreshold int           `yaml:"circuit_failure_threshold"`
	CircuitOpenDuration     time.Duration `yaml:"circuit_open_duration"`
	BulkheadConcurrency     int           `yaml:"bulkhead_concurrency"`
}

type Compliance struct {
	Profile                 string `yaml:"profile"` // standard | mlps3 | financial
	AuditRetentionDays      int    `yaml:"audit_retention_days"`
	NetworkLogRetentionDays int    `yaml:"network_log_retention_days"`
	SensitiveDataMasking    bool   `yaml:"sensitive_data_masking"`
	DisableDebugEndpoints   bool   `yaml:"disable_debug_endpoints"`
}

type Features struct {
	Flags map[string]bool `yaml:"flags"`
}

func Default() Config {
	return Config{
		App: App{Name: "Velora", Environment: "development"},
		Server: Server{
			ListenAddr: ":8080", GRPCListenAddr: ":9090", PublicURL: "http://localhost:8080", WebDir: "./web/dist",
			ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
			ShutdownTimeout: 15 * time.Second, MaxBodyBytes: 8 << 20,
		},
		// Schema changes are a release operation, not an API/worker start-up
		// side effect. Development profiles may explicitly enable this for a
		// disposable local database, but the safe default must be disabled.
		Database: Database{Provider: "postgres", MaxOpenConns: 30, MaxIdleConns: 10, MaxLifetime: 30 * time.Minute, QueryTimeout: 10 * time.Second, AutoMigrate: false},
		Cache:    Cache{Provider: "memory", Mode: "standalone", Prefix: "velora:", TTL: 10 * time.Minute},
		Messaging: Messaging{
			Provider: "disabled", RocketMQBatchSize: 16,
			RocketMQInvisibleDuration: 30 * time.Second, RocketMQAwaitDuration: 5 * time.Second,
		},
		Streaming:    Streaming{Provider: "disabled", ClientID: "forge"},
		Search:       Search{Provider: "disabled"},
		Storage:      Storage{Provider: "local", LocalRoot: "./data"},
		Discovery:    Discovery{Provider: "disabled", Group: "DEFAULT_GROUP", Cluster: "DEFAULT", Weight: 1, Metadata: map[string]string{}},
		RemoteConfig: RemoteConfig{Provider: "disabled", Group: "DEFAULT_GROUP"},
		Security: Security{
			AuthMode: "password", SessionTTL: 12 * time.Hour, SameSite: "lax", PasswordMinLength: 12,
			PasswordUpper: true, PasswordLower: true, PasswordDigit: true, PasswordSymbol: true,
			PasswordHistory: 5, PasswordMaxAgeDay: 90, LoginMaxFailures: 5, LoginLockDuration: 30 * time.Minute,
			CryptoProvider: "standard", CryptoAdapter: "software", CryptoKeyVersion: "v1",
		},
		Observability: Observability{LogLevel: "info", LogFormat: "json", MetricsEnabled: true, MetricsPath: "/metrics"},
		Resilience:    Resilience{DependencyTimeout: 10 * time.Second, RetryMaxAttempts: 3, RetryBaseDelay: 100 * time.Millisecond, CircuitFailureThreshold: 5, CircuitOpenDuration: 30 * time.Second, BulkheadConcurrency: 100},
		Compliance:    Compliance{Profile: "standard", AuditRetentionDays: 365, NetworkLogRetentionDays: 183, SensitiveDataMasking: true, DisableDebugEndpoints: true},
		Features:      Features{Flags: map[string]bool{}},
	}
}

// Load reads optional YAML, applies secret/env overrides, and validates.
// VELORA_CONFIG points to the local bootstrap YAML.
func Load() (Config, error) {
	cfg := Default()
	if p := strings.TrimSpace(os.Getenv("VELORA_CONFIG")); p != "" {
		b, err := securefile.Read(p)
		if err != nil {
			return cfg, fmt.Errorf("config read: %w", err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("config parse: %w", err)
		}
	}
	ApplyEnvironment(&cfg)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	if err := cfg.ValidateProductionAuth(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// LoadForMigration loads only the local/bootstrap configuration and validates
// the database subset. It intentionally ignores service discovery, messaging,
// search and other runtime-only requirements so a release migration job can run
// before application Pods are started. Database secrets still must come from
// environment variables or *_FILE.
func LoadForMigration() (Config, error) {
	cfg := Default()
	if p := strings.TrimSpace(os.Getenv("VELORA_CONFIG")); p != "" {
		b, err := securefile.Read(p)
		if err != nil {
			return cfg, fmt.Errorf("config read: %w", err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("config parse: %w", err)
		}
	}
	ApplyEnvironment(&cfg)
	if err := cfg.ValidateDatabase(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) ValidateDatabase() error {
	var errs []string
	switch c.Database.Provider {
	case "postgres", "mysql", "oceanbase":
	default:
		errs = append(errs, "database.provider must be postgres|mysql|oceanbase")
	}
	if c.Database.DSN == "" {
		errs = append(errs, "VELORA_DATABASE_DSN is required")
	}
	if c.Database.MaxOpenConns < 1 || c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, "database pool settings are invalid")
	}
	if isProduction(c.App.Environment) {
		if err := validateDatabaseTLS(c.Database.Provider, c.Database.DSN); err != nil {
			errs = append(errs, "database DSN: "+err.Error())
		}
		if c.Database.ReadOnlyDSN != "" {
			if err := validateDatabaseTLS(c.Database.Provider, c.Database.ReadOnlyDSN); err != nil {
				errs = append(errs, "read-only database DSN: "+err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// MergeYAML overlays non-secret remote configuration on an existing Config.
// Fields tagged yaml:"-" remain environment/file-only.
func MergeYAML(cfg *Config, raw []byte) error {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return fmt.Errorf("remote config parse: %w", err)
	}
	ApplyEnvironment(cfg) // local secret/env always wins over remote config
	if err := cfg.Validate(); err != nil {
		return err
	}
	return cfg.ValidateProductionAuth()
}

// ApplyEnvironment makes deployment/runtime values authoritative over YAML/Nacos.
func ApplyEnvironment(cfg *Config) {
	cfg.Database.DSN = secret("VELORA_DATABASE_DSN")
	cfg.Database.ReadOnlyDSN = secret("VELORA_DATABASE_READONLY_DSN")
	cfg.Cache.Username = secret("VELORA_REDIS_USERNAME")
	cfg.Cache.Password = secret("VELORA_REDIS_PASSWORD")
	cfg.Messaging.RocketMQAccessKey = secret("VELORA_ROCKETMQ_ACCESS_KEY")
	cfg.Messaging.RocketMQSecretKey = secret("VELORA_ROCKETMQ_SECRET_KEY")
	cfg.Streaming.Username = secret("VELORA_KAFKA_USERNAME")
	cfg.Streaming.Password = secret("VELORA_KAFKA_PASSWORD")
	cfg.Search.Username = secret("VELORA_SEARCH_USERNAME")
	cfg.Search.Password = secret("VELORA_SEARCH_PASSWORD")
	cfg.Storage.AccessKey = secret("VELORA_STORAGE_ACCESS_KEY")
	cfg.Storage.SecretKey = secret("VELORA_STORAGE_SECRET_KEY")
	cfg.Storage.SessionToken = secret("VELORA_STORAGE_SESSION_TOKEN")
	cfg.Discovery.Username = secret("VELORA_NACOS_USERNAME")
	cfg.Discovery.Password = secret("VELORA_NACOS_PASSWORD")
	cfg.RemoteConfig.Username = secret("VELORA_NACOS_USERNAME")
	cfg.RemoteConfig.Password = secret("VELORA_NACOS_PASSWORD")
	cfg.Security.CryptoKey = secret("VELORA_CRYPTO_KEY")
	cfg.Security.OIDCClientSecret = secret("VELORA_OIDC_CLIENT_SECRET")
	cfg.Security.CasdoorAutomationToken = secret("VELORA_CASDOOR_AUTOMATION_TOKEN")
	cfg.Security.CasdoorIdentityClientSecret = secret("VELORA_CASDOOR_IDENTITY_CLIENT_SECRET")
	cfg.Security.TurnstileSecret = secret("VELORA_TURNSTILE_SECRET")
	cfg.Provisioning.SpectraSecret = secret("VELORA_PROVISIONING_SPECTRA_SECRET")

	overrideString(&cfg.App.Name, "VELORA_APP_NAME")
	overrideString(&cfg.App.Environment, "VELORA_ENV")
	overrideString(&cfg.App.Region, "VELORA_REGION")
	overrideString(&cfg.App.Zone, "VELORA_ZONE")

	overrideString(&cfg.Server.ListenAddr, "VELORA_SERVER_LISTEN")
	overrideString(&cfg.Server.GRPCListenAddr, "VELORA_GRPC_LISTEN")
	overrideString(&cfg.Server.PublicURL, "VELORA_PUBLIC_URL")
	overrideString(&cfg.Server.WebDir, "VELORA_WEB_DIR")
	overrideCSV(&cfg.Server.WebCSPFrameSources, "VELORA_WEB_CSP_FRAME_SOURCES")
	overrideCSV(&cfg.Server.WebCSPConnectSources, "VELORA_WEB_CSP_CONNECT_SOURCES")
	overrideBool(&cfg.Server.WebCSPWujieEnabled, "VELORA_WEB_CSP_WUJIE_ENABLED")
	overrideString(&cfg.Server.WebCSPWujieApprovalRef, "VELORA_WEB_CSP_WUJIE_APPROVAL_REF")
	overrideBool(&cfg.Server.TLSEnabled, "VELORA_TLS_ENABLED")
	overrideString(&cfg.Server.TLSCertFile, "VELORA_TLS_CERT_FILE")
	overrideString(&cfg.Server.TLSKeyFile, "VELORA_TLS_KEY_FILE")
	overrideString(&cfg.Server.TLSClientCAFile, "VELORA_TLS_CLIENT_CA_FILE")
	overrideBool(&cfg.Server.RequireClientTLS, "VELORA_TLS_REQUIRE_CLIENT_CERT")

	overrideString(&cfg.Database.Provider, "VELORA_DATABASE_PROVIDER")
	overrideInt(&cfg.Database.MaxOpenConns, "VELORA_DATABASE_MAX_OPEN_CONNS")
	overrideInt(&cfg.Database.MaxIdleConns, "VELORA_DATABASE_MAX_IDLE_CONNS")
	overrideDuration(&cfg.Database.MaxLifetime, "VELORA_DATABASE_MAX_LIFETIME")
	overrideDuration(&cfg.Database.QueryTimeout, "VELORA_DATABASE_QUERY_TIMEOUT")
	overrideBool(&cfg.Database.AutoMigrate, "VELORA_DATABASE_AUTO_MIGRATE")

	overrideString(&cfg.Cache.Provider, "VELORA_CACHE_PROVIDER")
	overrideString(&cfg.Cache.Mode, "VELORA_REDIS_MODE")
	overrideCSV(&cfg.Cache.Addresses, "VELORA_REDIS_ADDRESSES")
	// backward-compatible singular address
	if v := strings.TrimSpace(os.Getenv("VELORA_REDIS_ADDRESS")); v != "" {
		cfg.Cache.Addresses = []string{v}
	}
	overrideString(&cfg.Cache.MasterName, "VELORA_REDIS_MASTER_NAME")
	overrideBool(&cfg.Cache.TLS, "VELORA_REDIS_TLS")
	overrideString(&cfg.Cache.TLSCAFile, "VELORA_REDIS_TLS_CA_FILE")
	overrideString(&cfg.Cache.TLSCertFile, "VELORA_REDIS_TLS_CERT_FILE")
	overrideString(&cfg.Cache.TLSKeyFile, "VELORA_REDIS_TLS_KEY_FILE")
	overrideString(&cfg.Cache.TLSServerName, "VELORA_REDIS_TLS_SERVER_NAME")
	overrideDuration(&cfg.Cache.TTL, "VELORA_CACHE_DEFAULT_TTL")

	overrideString(&cfg.Messaging.Provider, "VELORA_MESSAGING_PROVIDER")
	overrideBool(&cfg.Messaging.TLS, "VELORA_ROCKETMQ_TLS")
	overrideString(&cfg.Messaging.TLSCAFile, "VELORA_ROCKETMQ_TLS_CA_FILE")
	overrideString(&cfg.Messaging.TLSCertFile, "VELORA_ROCKETMQ_TLS_CERT_FILE")
	overrideString(&cfg.Messaging.TLSKeyFile, "VELORA_ROCKETMQ_TLS_KEY_FILE")
	overrideString(&cfg.Messaging.TLSServerName, "VELORA_ROCKETMQ_TLS_SERVER_NAME")
	overrideString(&cfg.Messaging.RocketMQEndpoint, "VELORA_ROCKETMQ_ENDPOINT")
	overrideString(&cfg.Messaging.RocketMQGroup, "VELORA_ROCKETMQ_GROUP")
	overrideString(&cfg.Messaging.RocketMQNamespace, "VELORA_ROCKETMQ_NAMESPACE")
	overrideCSV(&cfg.Messaging.RocketMQTopics, "VELORA_ROCKETMQ_TOPICS")
	overrideInt(&cfg.Messaging.RocketMQBatchSize, "VELORA_ROCKETMQ_BATCH_SIZE")
	overrideDuration(&cfg.Messaging.RocketMQInvisibleDuration, "VELORA_ROCKETMQ_INVISIBLE_DURATION")
	overrideDuration(&cfg.Messaging.RocketMQAwaitDuration, "VELORA_ROCKETMQ_AWAIT_DURATION")

	overrideString(&cfg.Streaming.Provider, "VELORA_STREAMING_PROVIDER")
	overrideCSV(&cfg.Streaming.Brokers, "VELORA_KAFKA_BROKERS")
	overrideString(&cfg.Streaming.ClientID, "VELORA_KAFKA_CLIENT_ID")
	overrideBool(&cfg.Streaming.TLS, "VELORA_KAFKA_TLS")
	overrideString(&cfg.Streaming.TLSCAFile, "VELORA_KAFKA_TLS_CA_FILE")
	overrideString(&cfg.Streaming.TLSCertFile, "VELORA_KAFKA_TLS_CERT_FILE")
	overrideString(&cfg.Streaming.TLSKeyFile, "VELORA_KAFKA_TLS_KEY_FILE")
	overrideString(&cfg.Streaming.TLSServerName, "VELORA_KAFKA_TLS_SERVER_NAME")

	overrideString(&cfg.Search.Provider, "VELORA_SEARCH_PROVIDER")
	overrideCSV(&cfg.Search.URLs, "VELORA_SEARCH_URLS")
	overrideBool(&cfg.Search.TLS, "VELORA_SEARCH_TLS")
	overrideString(&cfg.Search.TLSCAFile, "VELORA_SEARCH_TLS_CA_FILE")
	overrideString(&cfg.Search.TLSCertFile, "VELORA_SEARCH_TLS_CERT_FILE")
	overrideString(&cfg.Search.TLSKeyFile, "VELORA_SEARCH_TLS_KEY_FILE")
	overrideString(&cfg.Search.TLSServerName, "VELORA_SEARCH_TLS_SERVER_NAME")

	overrideString(&cfg.Storage.Provider, "VELORA_STORAGE_PROVIDER")
	overrideString(&cfg.Storage.Endpoint, "VELORA_STORAGE_ENDPOINT")
	overrideString(&cfg.Storage.Region, "VELORA_STORAGE_REGION")
	overrideString(&cfg.Storage.Bucket, "VELORA_STORAGE_BUCKET")
	overrideString(&cfg.Storage.Prefix, "VELORA_STORAGE_PREFIX")
	overrideString(&cfg.Storage.LocalRoot, "VELORA_STORAGE_LOCAL_ROOT")
	overrideBool(&cfg.Storage.PathStyle, "VELORA_STORAGE_PATH_STYLE")
	overrideBool(&cfg.Storage.TLS, "VELORA_STORAGE_TLS")
	overrideString(&cfg.Storage.SSEMode, "VELORA_STORAGE_SSE_MODE")
	overrideString(&cfg.Storage.SSEKMSKeyID, "VELORA_STORAGE_SSE_KMS_KEY_ID")
	overrideString(&cfg.Storage.TLSCAFile, "VELORA_STORAGE_TLS_CA_FILE")
	overrideString(&cfg.Storage.TLSCertFile, "VELORA_STORAGE_TLS_CERT_FILE")
	overrideString(&cfg.Storage.TLSKeyFile, "VELORA_STORAGE_TLS_KEY_FILE")
	overrideString(&cfg.Storage.TLSServerName, "VELORA_STORAGE_TLS_SERVER_NAME")
	overrideString(&cfg.Storage.CapabilityContractFile, "VELORA_STORAGE_CAPABILITY_CONTRACT_FILE")

	overrideString(&cfg.Discovery.Provider, "VELORA_DISCOVERY_PROVIDER")
	overrideCSV(&cfg.Discovery.Servers, "VELORA_NACOS_SERVERS")
	overrideString(&cfg.Discovery.Namespace, "VELORA_NACOS_NAMESPACE")
	overrideString(&cfg.Discovery.Group, "VELORA_NACOS_GROUP")
	overrideString(&cfg.Discovery.Cluster, "VELORA_NACOS_CLUSTER")
	overrideString(&cfg.Discovery.ServiceName, "VELORA_DISCOVERY_SERVICE_NAME")
	overrideString(&cfg.Discovery.GRPCServiceName, "VELORA_DISCOVERY_GRPC_SERVICE_NAME")
	overrideString(&cfg.Discovery.AdvertiseIP, "VELORA_DISCOVERY_ADVERTISE_IP")
	overrideUint64(&cfg.Discovery.AdvertisePort, "VELORA_DISCOVERY_ADVERTISE_PORT")
	overrideUint64(&cfg.Discovery.AdvertiseGRPCPort, "VELORA_DISCOVERY_ADVERTISE_GRPC_PORT")
	overrideBool(&cfg.Discovery.TLSRequired, "VELORA_NACOS_TLS_REQUIRED")
	overrideString(&cfg.Discovery.TLSCAFile, "VELORA_NACOS_TLS_CA_FILE")
	overrideString(&cfg.Discovery.TLSCertFile, "VELORA_NACOS_TLS_CERT_FILE")
	overrideString(&cfg.Discovery.TLSKeyFile, "VELORA_NACOS_TLS_KEY_FILE")
	overrideString(&cfg.Discovery.TLSServerName, "VELORA_NACOS_TLS_SERVER_NAME")

	overrideString(&cfg.RemoteConfig.Provider, "VELORA_REMOTE_CONFIG_PROVIDER")
	overrideCSV(&cfg.RemoteConfig.Servers, "VELORA_NACOS_SERVERS")
	overrideString(&cfg.RemoteConfig.Namespace, "VELORA_NACOS_NAMESPACE")
	overrideString(&cfg.RemoteConfig.Group, "VELORA_NACOS_CONFIG_GROUP")
	overrideString(&cfg.RemoteConfig.DataID, "VELORA_NACOS_CONFIG_DATA_ID")
	overrideBool(&cfg.RemoteConfig.FailFast, "VELORA_REMOTE_CONFIG_FAIL_FAST")
	overrideBool(&cfg.RemoteConfig.TLSRequired, "VELORA_NACOS_TLS_REQUIRED")
	overrideString(&cfg.RemoteConfig.TLSCAFile, "VELORA_NACOS_TLS_CA_FILE")
	overrideString(&cfg.RemoteConfig.TLSCertFile, "VELORA_NACOS_TLS_CERT_FILE")
	overrideString(&cfg.RemoteConfig.TLSKeyFile, "VELORA_NACOS_TLS_KEY_FILE")
	overrideString(&cfg.RemoteConfig.TLSServerName, "VELORA_NACOS_TLS_SERVER_NAME")

	overrideDuration(&cfg.Security.SessionTTL, "VELORA_SESSION_TTL")
	overrideInt(&cfg.Security.PasswordMinLength, "VELORA_PASSWORD_MIN_LENGTH")
	overrideBool(&cfg.Security.PasswordUpper, "VELORA_PASSWORD_REQUIRE_UPPER")
	overrideBool(&cfg.Security.PasswordLower, "VELORA_PASSWORD_REQUIRE_LOWER")
	overrideBool(&cfg.Security.PasswordDigit, "VELORA_PASSWORD_REQUIRE_DIGIT")
	overrideBool(&cfg.Security.PasswordSymbol, "VELORA_PASSWORD_REQUIRE_SYMBOL")
	overrideInt(&cfg.Security.PasswordHistory, "VELORA_PASSWORD_HISTORY")
	overrideInt(&cfg.Security.PasswordMaxAgeDay, "VELORA_PASSWORD_MAX_AGE_DAYS")
	overrideInt(&cfg.Security.LoginMaxFailures, "VELORA_LOGIN_MAX_FAILURES")
	overrideDuration(&cfg.Security.LoginLockDuration, "VELORA_LOGIN_LOCK_DURATION")
	overrideString(&cfg.Security.CryptoProvider, "VELORA_CRYPTO_PROVIDER")
	overrideString(&cfg.Security.CryptoAdapter, "VELORA_CRYPTO_ADAPTER")
	overrideString(&cfg.Security.CryptoKeyVersion, "VELORA_CRYPTO_KEY_VERSION")
	overrideString(&cfg.Security.AuthMode, "VELORA_AUTH_MODE")
	overrideString(&cfg.Security.OIDCIssuer, "VELORA_OIDC_ISSUER")
	overrideString(&cfg.Security.OIDCName, "VELORA_OIDC_NAME")
	overrideString(&cfg.Security.OIDCClientID, "VELORA_OIDC_CLIENT_ID")
	overrideString(&cfg.Security.OIDCRedirectURL, "VELORA_OIDC_REDIRECT_URL")
	overrideString(&cfg.Security.OIDCPostLogoutRedirectURL, "VELORA_OIDC_POST_LOGOUT_REDIRECT_URL")
	overrideString(&cfg.Security.CasdoorAccountURL, "VELORA_CASDOOR_ACCOUNT_URL")
	overrideString(&cfg.Security.CasdoorAdminURL, "VELORA_CASDOOR_ADMIN_URL")
	overrideCSV(&cfg.Security.CasdoorAllowedHosts, "VELORA_CASDOOR_ALLOWED_HOSTS")
	overrideBool(&cfg.Security.ApplicationOnboardingV2, "VELORA_APPLICATION_ONBOARDING_V2")
	overrideBool(&cfg.Security.CasdoorAdminEntryEnabled, "VELORA_CASDOOR_ADMIN_ENTRY_ENABLED")
	overrideBool(&cfg.Security.CasdoorApplicationAutomationEnabled, "VELORA_CASDOOR_APPLICATION_AUTOMATION_ENABLED")
	overrideBool(&cfg.Security.CasdoorIdentityManagementEnabled, "VELORA_CASDOOR_IDENTITY_MANAGEMENT_ENABLED")
	overrideString(&cfg.Security.CasdoorIdentityClientID, "VELORA_CASDOOR_IDENTITY_CLIENT_ID")
	overrideBool(&cfg.Security.CasdoorPasswordLoginEnabled, "VELORA_CASDOOR_PASSWORD_LOGIN_ENABLED")
	overrideString(&cfg.Security.CasdoorApplication, "VELORA_CASDOOR_APPLICATION")
	overrideString(&cfg.Security.CasdoorOrganization, "VELORA_CASDOOR_ORGANIZATION")
	overrideString(&cfg.Security.TurnstileSiteKey, "VELORA_TURNSTILE_SITE_KEY")
	overrideCSV(&cfg.Security.TurnstileHostnames, "VELORA_TURNSTILE_HOSTNAMES")
	overrideString(&cfg.Security.TurnstileAction, "VELORA_TURNSTILE_ACTION")
	overrideString(&cfg.Security.OIDCInternalURL, "VELORA_OIDC_INTERNAL_URL")
	overrideBool(&cfg.Security.OIDCProviderEnabled, "VELORA_OIDC_PROVIDER_ENABLED")
	overrideBool(&cfg.Security.SecureCookies, "VELORA_SECURE_COOKIES")
	overrideBool(&cfg.Provisioning.SpectraEnabled, "VELORA_PROVISIONING_SPECTRA_ENABLED")
	overrideString(&cfg.Provisioning.SpectraURL, "VELORA_PROVISIONING_SPECTRA_URL")
	overrideString(&cfg.Security.SameSite, "VELORA_SAME_SITE")
	overrideCSV(&cfg.Security.AllowedOrigins, "VELORA_ALLOWED_ORIGINS")
	overrideCSV(&cfg.Security.TrustedProxies, "VELORA_TRUSTED_PROXIES")

	overrideString(&cfg.Observability.LogLevel, "VELORA_LOG_LEVEL")
	overrideString(&cfg.Observability.LogFormat, "VELORA_LOG_FORMAT")
	overrideString(&cfg.Observability.MetricsPath, "VELORA_METRICS_PATH")
	overrideString(&cfg.Observability.OTLPEndpoint, "VELORA_OTLP_ENDPOINT")
	overrideBool(&cfg.Observability.OTLPTLS, "VELORA_OTLP_TLS")
	overrideString(&cfg.Observability.OTLPTLSCAFile, "VELORA_OTLP_TLS_CA_FILE")
	overrideString(&cfg.Observability.OTLPTLSCertFile, "VELORA_OTLP_TLS_CERT_FILE")
	overrideString(&cfg.Observability.OTLPTLSKeyFile, "VELORA_OTLP_TLS_KEY_FILE")
	overrideString(&cfg.Observability.OTLPTLSServerName, "VELORA_OTLP_TLS_SERVER_NAME")
	overrideBool(&cfg.Observability.TracingEnabled, "VELORA_TRACING_ENABLED")
	overrideBool(&cfg.Observability.MetricsEnabled, "VELORA_METRICS_ENABLED")
	overrideBool(&cfg.Observability.PprofEnabled, "VELORA_PPROF_ENABLED")

	overrideDuration(&cfg.Resilience.DependencyTimeout, "VELORA_DEPENDENCY_TIMEOUT")
	overrideInt(&cfg.Resilience.RetryMaxAttempts, "VELORA_RETRY_MAX_ATTEMPTS")
	overrideDuration(&cfg.Resilience.RetryBaseDelay, "VELORA_RETRY_BASE_DELAY")
	overrideInt(&cfg.Resilience.CircuitFailureThreshold, "VELORA_CIRCUIT_FAILURE_THRESHOLD")
	overrideDuration(&cfg.Resilience.CircuitOpenDuration, "VELORA_CIRCUIT_OPEN_DURATION")
	overrideInt(&cfg.Resilience.BulkheadConcurrency, "VELORA_BULKHEAD_CONCURRENCY")

	overrideString(&cfg.Compliance.Profile, "VELORA_COMPLIANCE_PROFILE")
	overrideInt(&cfg.Compliance.AuditRetentionDays, "VELORA_AUDIT_RETENTION_DAYS")
	overrideInt(&cfg.Compliance.NetworkLogRetentionDays, "VELORA_NETWORK_LOG_RETENTION_DAYS")
	overrideBool(&cfg.Compliance.SensitiveDataMasking, "VELORA_SENSITIVE_DATA_MASKING")
	overrideBool(&cfg.Compliance.DisableDebugEndpoints, "VELORA_DISABLE_DEBUG_ENDPOINTS")
}

func normalizeStorageProvider(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "", "local":
		return "local"
	case "s3", "s3-compatible", "s3_compatible", "minio", "minio-s3", "oss", "cos", "ceph", "ceph-rgw", "radosgw":
		return "s3"
	default:
		return v
	}
}

func (c Config) Validate() error {
	var errs []string
	switch c.Database.Provider {
	case "postgres", "mysql", "oceanbase":
	default:
		errs = append(errs, "database.provider must be postgres|mysql|oceanbase")
	}
	if c.Database.DSN == "" {
		errs = append(errs, "VELORA_DATABASE_DSN is required")
	}
	if isProduction(c.App.Environment) {
		if err := validateDatabaseTLS(c.Database.Provider, c.Database.DSN); err != nil {
			errs = append(errs, "database DSN: "+err.Error())
		}
		if c.Database.ReadOnlyDSN != "" {
			if err := validateDatabaseTLS(c.Database.Provider, c.Database.ReadOnlyDSN); err != nil {
				errs = append(errs, "read-only database DSN: "+err.Error())
			}
		}
	}
	switch c.Cache.Provider {
	case "disabled", "memory", "redis":
	default:
		errs = append(errs, "cache.provider must be disabled|memory|redis")
	}
	if c.Cache.Provider == "redis" && len(c.Cache.Addresses) == 0 {
		errs = append(errs, "cache.addresses is required for redis")
	}
	if c.Cache.Provider == "redis" && isProduction(c.App.Environment) && !c.Cache.TLS {
		errs = append(errs, "cache.tls must be enabled for redis in production")
	}
	if isProduction(c.App.Environment) && c.Cache.Provider != "redis" {
		errs = append(errs, "cache.provider must be redis in production; memory/disabled cache cannot provide shared session and rate-limit state")
	}
	if c.Cache.Mode != "" && c.Cache.Mode != "standalone" && c.Cache.Mode != "sentinel" && c.Cache.Mode != "cluster" {
		errs = append(errs, "cache.mode must be standalone|sentinel|cluster")
	}
	switch c.Messaging.Provider {
	case "disabled", "rocketmq":
	default:
		errs = append(errs, "messaging.provider must be disabled|rocketmq")
	}
	if c.Messaging.Provider == "rocketmq" && c.Messaging.RocketMQEndpoint == "" {
		errs = append(errs, "messaging.rocketmq_endpoint required for rocketmq")
	}
	if c.Messaging.Provider == "rocketmq" && len(c.Messaging.RocketMQTopics) == 0 {
		errs = append(errs, "messaging.rocketmq_topics required for rocketmq")
	}
	if c.Messaging.Provider == "rocketmq" && (c.Messaging.RocketMQAccessKey == "" || c.Messaging.RocketMQSecretKey == "") {
		errs = append(errs, "VELORA_ROCKETMQ_ACCESS_KEY and VELORA_ROCKETMQ_SECRET_KEY are required for rocketmq")
	}
	if isProduction(c.App.Environment) && c.Messaging.Provider == "rocketmq" && !c.Messaging.TLS {
		errs = append(errs, "messaging.tls must be enabled in production")
	}
	switch c.Streaming.Provider {
	case "disabled", "kafka":
	default:
		errs = append(errs, "streaming.provider must be disabled|kafka")
	}
	if c.Streaming.Provider == "kafka" && len(c.Streaming.Brokers) == 0 {
		errs = append(errs, "streaming.brokers required for kafka")
	}
	if isProduction(c.App.Environment) && c.Streaming.Provider == "kafka" && !c.Streaming.TLS {
		errs = append(errs, "streaming.tls must be enabled in production")
	}
	switch c.Search.Provider {
	case "disabled", "elasticsearch", "opensearch":
	default:
		errs = append(errs, "search.provider must be disabled|elasticsearch|opensearch")
	}
	if c.Search.Provider != "disabled" && len(c.Search.URLs) == 0 {
		errs = append(errs, "search.urls required when search is enabled")
	}
	if isProduction(c.App.Environment) && c.Search.Provider != "disabled" {
		if !c.Search.TLS {
			errs = append(errs, "search.tls must be enabled in production")
		}
		for _, endpoint := range c.Search.URLs {
			if !isHTTPSURL(endpoint) {
				errs = append(errs, "search.urls must use https in production")
				break
			}
		}
	}
	switch c.Storage.Provider {
	case "local", "s3", "s3-compatible", "s3_compatible", "minio", "minio-s3", "oss", "cos", "ceph", "ceph-rgw", "radosgw":
	default:
		errs = append(errs, "storage.provider must be local|s3 (s3-compatible aliases: s3-compatible/minio/oss/cos/ceph)")
	}
	if normalizeStorageProvider(c.Storage.Provider) == "s3" && c.Storage.Bucket == "" {
		errs = append(errs, "storage.bucket required for s3")
	}
	if isProduction(c.App.Environment) && normalizeStorageProvider(c.Storage.Provider) == "s3" {
		if !c.Storage.TLS {
			errs = append(errs, "storage.tls must be enabled for s3 in production")
		}
		if strings.TrimSpace(c.Storage.Endpoint) != "" && !isHTTPSURL(c.Storage.Endpoint) {
			errs = append(errs, "storage.endpoint must use https in production")
		}
		if strings.TrimSpace(c.Storage.AccessKey) == "" || strings.TrimSpace(c.Storage.SecretKey) == "" {
			errs = append(errs, "storage access_key and secret_key are required in production")
		}
	}
	if normalizeStorageProvider(c.Storage.Provider) == "local" && strings.TrimSpace(c.Storage.LocalRoot) == "" {
		errs = append(errs, "storage.local_root required for local storage")
	}
	if isProduction(c.App.Environment) && normalizeStorageProvider(c.Storage.Provider) == "local" {
		errs = append(errs, "storage.provider must be an S3-compatible target in production")
	}
	switch strings.ToLower(strings.TrimSpace(c.Storage.SSEMode)) {
	case "", "none", "s3", "kms":
	default:
		errs = append(errs, "storage.sse_mode must be none|s3|kms")
	}
	if strings.EqualFold(strings.TrimSpace(c.Storage.SSEMode), "kms") && strings.TrimSpace(c.Storage.SSEKMSKeyID) == "" {
		errs = append(errs, "storage.sse_kms_key_id is required for sse_mode=kms")
	}
	switch c.Discovery.Provider {
	case "disabled", "nacos":
	default:
		errs = append(errs, "discovery.provider must be disabled|nacos")
	}
	if c.Discovery.Provider == "nacos" {
		if len(c.Discovery.Servers) == 0 {
			errs = append(errs, "discovery.servers required for nacos")
		}
		if c.Discovery.AdvertiseIP == "" || c.Discovery.AdvertisePort == 0 || c.Discovery.AdvertiseGRPCPort == 0 {
			errs = append(errs, "nacos discovery requires advertise_ip, advertise_port and advertise_grpc_port")
		}
		if c.Discovery.ServiceName != "" && c.Discovery.ServiceName == c.Discovery.GRPCServiceName {
			errs = append(errs, "nacos HTTP and gRPC service names must differ")
		}
	}
	switch c.RemoteConfig.Provider {
	case "disabled", "nacos":
	default:
		errs = append(errs, "remote_config.provider must be disabled|nacos")
	}
	if c.RemoteConfig.Provider == "nacos" && (len(c.RemoteConfig.Servers) == 0 || c.RemoteConfig.DataID == "") {
		errs = append(errs, "nacos remote config requires servers and data_id")
	}
	if c.Security.PasswordMinLength < 8 {
		errs = append(errs, "security.password_min_length must be >= 8")
	}
	if c.Security.PasswordHistory < 0 || c.Security.PasswordHistory > 24 {
		errs = append(errs, "security.password_history must be 0..24")
	}
	switch c.Security.CryptoProvider {
	case "standard", "gm":
	default:
		errs = append(errs, "security.crypto_provider must be standard|gm")
	}
	switch strings.ToLower(strings.TrimSpace(c.Security.CryptoAdapter)) {
	case "software", "kms", "hsm", "pkcs11":
	default:
		errs = append(errs, "security.crypto_adapter must be software|kms|hsm|pkcs11")
	}
	switch strings.ToLower(strings.TrimSpace(c.Security.AuthMode)) {
	case "oidc", "password":
	default:
		errs = append(errs, "security.auth_mode must be oidc|password")
	}
	if c.Security.CasdoorPasswordLoginEnabled {
		if strings.ToLower(strings.TrimSpace(c.Security.AuthMode)) != "oidc" {
			errs = append(errs, "security.casdoor_password_login_enabled requires auth_mode=oidc")
		}
		if strings.TrimSpace(c.Security.CasdoorApplication) == "" {
			errs = append(errs, "security.casdoor_application is required when Casdoor password login is enabled")
		}
		if strings.TrimSpace(c.Security.CasdoorOrganization) == "" {
			errs = append(errs, "security.casdoor_organization is required when Casdoor password login is enabled")
		}
	}
	turnstileParts := []string{strings.TrimSpace(c.Security.TurnstileSiteKey), strings.TrimSpace(c.Security.TurnstileSecret)}
	turnstileConfigured := c.Security.TurnstileConfigured()
	// Development may carry a public site key before the secret is retrieved;
	// the verifier remains disabled until all values are present. Production
	// rejects every partial configuration so a protected form cannot start in
	// an unverified state.
	if isProduction(c.App.Environment) && (turnstileConfigured || turnstileParts[0] != "" || turnstileParts[1] != "" || len(c.Security.TurnstileHostnames) > 0) {
		if turnstileParts[0] == "" {
			errs = append(errs, "security.turnstile_site_key is required when Turnstile is configured")
		}
		if turnstileParts[1] == "" {
			errs = append(errs, "VELORA_TURNSTILE_SECRET is required when Turnstile is configured")
		}
		if len(c.Security.TurnstileHostnames) == 0 {
			errs = append(errs, "security.turnstile_hostnames is required when Turnstile is configured")
		}
		seenHostnames := make(map[string]struct{}, len(c.Security.TurnstileHostnames))
		for _, raw := range c.Security.TurnstileHostnames {
			host := strings.ToLower(strings.TrimSpace(raw))
			if host == "" || strings.ContainsAny(host, "/\\,:;?'\" ") {
				errs = append(errs, "security.turnstile_hostnames must contain hostnames without schemes, ports, or paths")
				continue
			}
			if _, duplicate := seenHostnames[host]; duplicate {
				errs = append(errs, "security.turnstile_hostnames must not contain duplicates")
			}
			seenHostnames[host] = struct{}{}
			if isProduction(c.App.Environment) && (host == "localhost" || host == "127.0.0.1") {
				errs = append(errs, "security.turnstile_hostnames must not include localhost or 127.0.0.1 in production")
			}
		}
		action := c.Security.EffectiveTurnstileAction()
		if len(action) > 32 || strings.ContainsAny(action, " ,;:/\\") {
			errs = append(errs, "security.turnstile_action must be 1..32 characters without separators")
		}
	}
	if strings.EqualFold(c.Security.SameSite, "none") && !c.Security.SecureCookies {
		errs = append(errs, "security.same_site=none requires secure_cookies=true")
	}

	for name, pair := range map[string][2]string{
		"redis":     {c.Cache.TLSCertFile, c.Cache.TLSKeyFile},
		"messaging": {c.Messaging.TLSCertFile, c.Messaging.TLSKeyFile},
		"streaming": {c.Streaming.TLSCertFile, c.Streaming.TLSKeyFile},
		"search":    {c.Search.TLSCertFile, c.Search.TLSKeyFile},
		"storage":   {c.Storage.TLSCertFile, c.Storage.TLSKeyFile},
		"otlp":      {c.Observability.OTLPTLSCertFile, c.Observability.OTLPTLSKeyFile},
	} {
		if (pair[0] == "") != (pair[1] == "") {
			errs = append(errs, name+" tls_cert_file and tls_key_file must be configured together")
		}
	}
	if c.Server.ListenAddr == "" {
		errs = append(errs, "server.listen_addr is required")
	}
	if c.Server.GRPCListenAddr == "" {
		errs = append(errs, "server.grpc_listen_addr is required")
	}
	if isProduction(c.App.Environment) {
		if !validProductionPublicURL(c.Server.PublicURL) {
			errs = append(errs, "server.public_url must be a non-loopback https URL in production")
		}
		if !c.Security.SecureCookies {
			errs = append(errs, "security.secure_cookies must be true in production")
		}
		if len(c.Security.AllowedOrigins) == 0 {
			errs = append(errs, "security.allowed_origins must contain at least one exact https origin in production")
		}
		seenOrigins := make(map[string]struct{}, len(c.Security.AllowedOrigins))
		for _, origin := range c.Security.AllowedOrigins {
			origin = strings.TrimSpace(origin)
			if !validWebCSPOrigin(origin, true) {
				errs = append(errs, "security.allowed_origins must contain exact https origins in production")
				continue
			}
			if _, duplicate := seenOrigins[origin]; duplicate {
				errs = append(errs, "security.allowed_origins must not contain duplicates")
			}
			seenOrigins[origin] = struct{}{}
		}
	}
	for _, proxy := range c.Security.TrustedProxies {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(proxy)); err != nil {
			errs = append(errs, "security.trusted_proxies must contain valid CIDRs")
		}
	}
	if c.Server.TLSEnabled && (c.Server.TLSCertFile == "" || c.Server.TLSKeyFile == "") {
		errs = append(errs, "tls enabled requires tls_cert_file and tls_key_file")
	}
	if c.Server.RequireClientTLS && c.Server.TLSClientCAFile == "" {
		errs = append(errs, "client mTLS requires tls_client_ca_file")
	}
	seenFrameSources := make(map[string]struct{}, len(c.Server.WebCSPFrameSources))
	for _, source := range c.Server.WebCSPFrameSources {
		source = strings.TrimSpace(source)
		if !validWebCSPOrigin(source, isProduction(c.App.Environment)) {
			errs = append(errs, "server.web_csp_frame_sources must contain exact HTTP(S) origins and use HTTPS in production")
			continue
		}
		if _, duplicate := seenFrameSources[source]; duplicate {
			errs = append(errs, "server.web_csp_frame_sources must not contain duplicates")
		}
		seenFrameSources[source] = struct{}{}
	}
	seenConnectSources := make(map[string]struct{}, len(c.Server.WebCSPConnectSources))
	for _, source := range c.Server.WebCSPConnectSources {
		source = strings.TrimSpace(source)
		if !validWebCSPOrigin(source, isProduction(c.App.Environment)) {
			errs = append(errs, "server.web_csp_connect_sources must contain exact HTTP(S) origins and use HTTPS in production")
			continue
		}
		if _, duplicate := seenConnectSources[source]; duplicate {
			errs = append(errs, "server.web_csp_connect_sources must not contain duplicates")
		}
		seenConnectSources[source] = struct{}{}
	}
	approvalRef := strings.TrimSpace(c.Server.WebCSPWujieApprovalRef)
	if c.Server.WebCSPWujieEnabled && !validApprovalReference(approvalRef) {
		errs = append(errs, "server.web_csp_wujie_enabled requires a valid web_csp_wujie_approval_ref")
	}
	if !c.Server.WebCSPWujieEnabled && approvalRef != "" {
		errs = append(errs, "server.web_csp_wujie_approval_ref requires web_csp_wujie_enabled=true")
	}
	if c.Observability.MetricsEnabled && !strings.HasPrefix(c.Observability.MetricsPath, "/") {
		errs = append(errs, "observability.metrics_path must start with /")
	}
	if c.Observability.TracingEnabled && strings.TrimSpace(c.Observability.OTLPEndpoint) == "" {
		errs = append(errs, "observability.otlp_endpoint is required when tracing is enabled")
	}
	if isProduction(c.App.Environment) && c.Observability.TracingEnabled {
		if !c.Observability.OTLPTLS {
			errs = append(errs, "observability.otlp_tls must be enabled in production")
		}
		if !isHTTPSURL(c.Observability.OTLPEndpoint) {
			errs = append(errs, "observability.otlp_endpoint must use https in production")
		}
	}
	if c.Resilience.RetryMaxAttempts < 1 || c.Resilience.CircuitFailureThreshold < 1 || c.Resilience.BulkheadConcurrency < 1 {
		errs = append(errs, "resilience values must be positive")
	}
	if c.Compliance.NetworkLogRetentionDays < 183 && (c.Compliance.Profile == "mlps3" || c.Compliance.Profile == "financial") {
		errs = append(errs, "mlps3/financial profile requires network_log_retention_days >= 183")
	}
	if isProduction(c.App.Environment) && c.Database.AutoMigrate {
		errs = append(errs, "database.auto_migrate must be false in production; run velora-migrate as a one-shot release job")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// ValidateProductionAuth is kept separate from the structural validator so
// unit tests can validate individual production controls without having to
// provide deployment secrets and IdP endpoints. Runtime startup always calls
// this gate after environment and remote configuration are merged.
func (c Config) ValidateProductionAuth() error {
	if !isProduction(c.App.Environment) {
		return nil
	}
	var errs []string
	if strings.ToLower(strings.TrimSpace(c.Security.AuthMode)) != "oidc" {
		errs = append(errs, "security.auth_mode must be oidc in production")
	}
	if strings.TrimSpace(c.Security.OIDCIssuer) == "" || !isHTTPSURL(c.Security.OIDCIssuer) {
		errs = append(errs, "security.oidc_issuer must be an https URL in production")
	}
	if name := strings.ToLower(strings.TrimSpace(c.Security.OIDCName)); name != "" && name != "casdoor" {
		errs = append(errs, "security.oidc_name must be casdoor in production")
	}
	if strings.TrimSpace(os.Getenv("VELORA_LDAP_URL")) != "" {
		errs = append(errs, "VELORA_LDAP_URL must be unset in production; Casdoor is the only identity provider")
	}
	if strings.TrimSpace(c.Security.OIDCClientID) == "" {
		errs = append(errs, "security.oidc_client_id is required in production")
	}
	if strings.TrimSpace(c.Security.OIDCClientSecret) == "" {
		errs = append(errs, "VELORA_OIDC_CLIENT_SECRET is required in production")
	}
	if strings.TrimSpace(c.Security.OIDCRedirectURL) == "" || !isHTTPSURL(c.Security.OIDCRedirectURL) {
		errs = append(errs, "security.oidc_redirect_url must be an https URL in production")
	}
	if c.Security.OIDCPostLogoutRedirectURL != "" && !isHTTPSURL(c.Security.OIDCPostLogoutRedirectURL) {
		errs = append(errs, "security.oidc_post_logout_redirect_url must be an https URL in production")
	}
	if c.Security.SessionTTL <= 0 || c.Security.SessionTTL > time.Hour {
		errs = append(errs, "security.session_ttl must be between 1 second and 1 hour for production OIDC")
	}
	if strings.TrimSpace(c.Security.CasdoorAccountURL) == "" || !isHTTPSURL(c.Security.CasdoorAccountURL) {
		errs = append(errs, "security.casdoor_account_url must be an https URL in production")
	}
	if c.Security.CasdoorAdminEntryEnabled || c.Security.CasdoorApplicationAutomationEnabled {
		if strings.TrimSpace(c.Security.CasdoorAdminURL) == "" || !isHTTPSURL(c.Security.CasdoorAdminURL) {
			errs = append(errs, "security.casdoor_admin_url must be an https URL when Casdoor administration is enabled")
		}
		adminURL, _ := url.Parse(c.Security.CasdoorAdminURL)
		if len(c.Security.CasdoorAllowedHosts) == 0 || adminURL == nil || !containsFold(c.Security.CasdoorAllowedHosts, adminURL.Hostname()) {
			errs = append(errs, "security.casdoor_allowed_hosts must contain the admin URL hostname")
		}
	}
	if c.Security.OIDCProviderEnabled {
		errs = append(errs, "security.oidc_provider_enabled must be false in production; Casdoor is the only OIDC provider")
	}
	if c.Security.CasdoorPasswordLoginEnabled && !c.Security.TurnstileConfigured() {
		errs = append(errs, "Velora-hosted password login requires Turnstile configuration in production")
	}
	// The standard single-node profile deliberately supports the Velora-owned
	// credential form required by the product. It remains safe only when the
	// complete Turnstile/OIDC/HTTPS/Redis/rate-limit gates above are satisfied.
	// Financial profiles keep the stricter policy and require the external IdP
	// browser flow instead of accepting credentials at the portal.
	financialProfile := c.Compliance.Profile == "financial" || c.Compliance.Profile == "mlps3"
	if c.Security.CasdoorPasswordLoginEnabled && financialProfile {
		errs = append(errs, "security.casdoor_password_login_enabled must be false in financial/mlps3 production profiles")
	}
	if len(c.Security.TrustedProxies) == 0 {
		errs = append(errs, "security.trusted_proxies must contain approved proxy CIDRs in production")
	}
	if strings.EqualFold(strings.TrimSpace(c.Security.CryptoAdapter), "software") && financialProfile {
		errs = append(errs, "security.crypto_adapter must be an approved KMS/HSM/PKCS#11 adapter in financial/mlps3 production profiles")
	}
	if strings.EqualFold(strings.TrimSpace(c.Security.CryptoProvider), "gm") && strings.EqualFold(strings.TrimSpace(c.Security.CryptoAdapter), "software") {
		errs = append(errs, "security.crypto_provider=gm requires an approved KMS/HSM/PKCS#11 adapter in production; software GM is not an approved trust root")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func validWebCSPOrigin(source string, production bool) bool {
	if source == "" || len(source) > 512 || strings.ContainsAny(source, "*'\";,") {
		return false
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return false
	}
	if production {
		return parsed.Scheme == "https"
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func validProductionPublicURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return false
	}
	return u.Path == "" || u.Path == "/"
}

func validApprovalReference(value string) bool {
	if len(value) < 6 || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("._:/-", char) {
			continue
		}
		return false
	}
	return true
}

func isProduction(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

func isHTTPSURL(value string) bool {
	u, err := url.Parse(strings.TrimSpace(value))
	return err == nil && strings.EqualFold(u.Scheme, "https") && u.Host != ""
}

func validateDatabaseTLS(provider, dsn string) error {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "postgres":
		mode, err := postgresSSLMode(dsn)
		if err != nil {
			return err
		}
		if mode != "verify-full" {
			return fmt.Errorf("PostgreSQL sslmode must be verify-full in production")
		}
	case "mysql", "oceanbase":
		mode, err := mysqlTLSMode(dsn)
		if err != nil {
			return err
		}
		if mode != "true" {
			return fmt.Errorf("MySQL-compatible tls must be true in production")
		}
	}
	return nil
}

func postgresSSLMode(dsn string) (string, error) {
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", fmt.Errorf("invalid PostgreSQL DSN")
		}
		return strings.ToLower(strings.TrimSpace(u.Query().Get("sslmode"))), nil
	}
	for _, field := range strings.Fields(dsn) {
		key, value, ok := strings.Cut(field, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "sslmode") {
			return strings.ToLower(strings.Trim(strings.TrimSpace(value), "'\"")), nil
		}
	}
	return "", nil
}

func mysqlTLSMode(dsn string) (string, error) {
	index := strings.LastIndexByte(dsn, '?')
	if index < 0 || index == len(dsn)-1 {
		return "", nil
	}
	query, err := url.ParseQuery(dsn[index+1:])
	if err != nil {
		return "", fmt.Errorf("invalid MySQL-compatible DSN parameters")
	}
	return strings.ToLower(strings.TrimSpace(query.Get("tls"))), nil
}

func secret(key string) string {
	if p := strings.TrimSpace(os.Getenv(key + "_FILE")); p != "" {
		b, err := securefile.Read(p)
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return strings.TrimSpace(os.Getenv(key))
}
func overrideString(dst *string, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		*dst = v
	}
}
func overrideCSV(dst *[]string, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		parts := strings.Split(v, ",")
		*dst = (*dst)[:0]
		for _, p := range parts {
			if x := strings.TrimSpace(p); x != "" {
				*dst = append(*dst, x)
			}
		}
	}
}
func overrideBool(dst *bool, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, e := strconv.ParseBool(v); e == nil {
			*dst = b
		}
	}
}
func overrideInt(dst *int, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			*dst = n
		}
	}
}
func overrideDuration(dst *time.Duration, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, e := time.ParseDuration(v); e == nil {
			*dst = d
		}
	}
}
func overrideUint64(dst *uint64, key string) {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, e := strconv.ParseUint(v, 10, 64); e == nil {
			*dst = n
		}
	}
}
