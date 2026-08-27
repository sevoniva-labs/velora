package bootstrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	ktracing "github.com/go-kratos/kratos/v2/middleware/tracing"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/adapters/auditsink"
	"github.com/sevoniva-labs/velora/server/internal/adapters/kratosapi"
	"github.com/sevoniva-labs/velora/server/internal/adapters/repository"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	appconfigchange "github.com/sevoniva-labs/velora/server/internal/app/configchange"
	appdatapolicy "github.com/sevoniva-labs/velora/server/internal/app/datapolicy"
	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
	appportal "github.com/sevoniva-labs/velora/server/internal/app/portal"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"github.com/sevoniva-labs/velora/server/internal/platform/authz"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/casdooradmin"
	"github.com/sevoniva-labs/velora/server/internal/platform/casdooridentity"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/credentialhandoff"
	"github.com/sevoniva-labs/velora/server/internal/platform/csrf"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/discovery"
	"github.com/sevoniva-labs/velora/server/internal/platform/health"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpserver"
	"github.com/sevoniva-labs/velora/server/internal/platform/idempotency"
	"github.com/sevoniva-labs/velora/server/internal/platform/logx"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
	"github.com/sevoniva-labs/velora/server/internal/platform/metrics"
	"github.com/sevoniva-labs/velora/server/internal/platform/observability"
	"github.com/sevoniva-labs/velora/server/internal/platform/provisioninghttp"
	"github.com/sevoniva-labs/velora/server/internal/platform/ratelimit"
	"github.com/sevoniva-labs/velora/server/internal/platform/remoteconfig"
	"github.com/sevoniva-labs/velora/server/internal/platform/search"
	"github.com/sevoniva-labs/velora/server/internal/platform/securefile"
	appcrypto "github.com/sevoniva-labs/velora/server/internal/platform/security/crypto"
	"github.com/sevoniva-labs/velora/server/internal/platform/storage"
	"github.com/sevoniva-labs/velora/server/internal/platform/turnstile"
)

type Options struct{ Version string }

type App struct {
	cfg           config.Config
	log           *slog.Logger
	runtime       *kratos.App
	db            *database.DB
	cache         cache.Cache
	bus           messaging.Bus
	registry      discovery.Registry
	traceShutdown observability.Shutdown
	portal        *kratosapi.PortalService
}

func New(ctx context.Context, opts Options) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Bootstrap remote config before constructing infrastructure. Environment
	// and *_FILE values are re-applied after the remote YAML so secrets and
	// emergency deployment overrides always win.
	if cfg.RemoteConfig.Provider != "" && cfg.RemoteConfig.Provider != "disabled" {
		src, err := remoteconfig.New(cfg.RemoteConfig)
		if err != nil {
			if cfg.RemoteConfig.FailFast {
				return nil, fmt.Errorf("remote config: %w", err)
			}
			slog.Warn("remote config client unavailable; using local config", "err", err)
		} else if raw, err := src.Load(ctx); err != nil {
			if cfg.RemoteConfig.FailFast {
				return nil, fmt.Errorf("remote config load: %w", err)
			}
			slog.Warn("remote config load failed; using local config", "err", err)
		} else if len(raw) > 0 {
			if err := config.MergeYAML(&cfg, raw); err != nil {
				return nil, err
			}
		}
	}
	oidcProviders, ldapProviders, err := newFederatedIdentityProviders(ctx, cfg)
	if err != nil {
		return nil, err
	}

	log := logx.New(cfg.Observability.LogLevel, cfg.Observability.LogFormat, cfg.App.Name, cfg.App.Environment, opts.Version)
	slog.SetDefault(log)

	traceShutdown, err := observability.InitTracing(ctx, cfg.Observability, cfg.App.Name, opts.Version, cfg.App.Environment)
	if err != nil {
		return nil, fmt.Errorf("tracing: %w", err)
	}

	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		_ = traceShutdown(context.Background())
		return nil, err
	}
	if cfg.Database.AutoMigrate {
		if err = database.Migrate(db.DB, cfg.Database.Provider); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	c, err := cache.New(cfg.Cache)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	bus, err := messaging.New(cfg.Messaging)
	if err != nil {
		_ = c.Close()
		_ = db.Close()
		return nil, err
	}
	se, err := search.New(cfg.Search)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, err
	}
	st, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, err
	}
	crypt, err := appcrypto.NewWithAdapter(cfg.Security.CryptoProvider, cfg.Security.CryptoAdapter, cfg.Security.CryptoKey, cfg.Security.CryptoKeyVersion)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, fmt.Errorf("crypto: %w", err)
	}

	reg, err := discovery.New(cfg.Discovery, cfg.App.Name, opts.Version, cfg.App.Environment)
	if err != nil {
		bus.Close()
		_ = c.Close()
		_ = db.Close()
		return nil, fmt.Errorf("service discovery: %w", err)
	}

	repo := repository.NewIdentityRepo(db)
	approvalSvc := appapproval.NewService(repository.NewApprovalRepo(db))
	configChangeSvc := appconfigchange.NewService(repository.NewConfigChangeRepo(db))
	dataPolicySvc := appdatapolicy.NewService(repository.NewDataPolicyRepo(db))
	portalSvc := appportal.NewService(repository.NewPortalRepo(db))
	portalSvc.ConfigureOIDCIssuer(cfg.Security.OIDCIssuer)
	provisioningCipher, err := appcrypto.NewEnvelopeCipher(crypt)
	if err != nil {
		return nil, fmt.Errorf("provisioning crypto: %w", err)
	}
	portalSvc.ConfigureProvisioningCipher(provisioningCipher)
	identitySvc := appidentity.NewService(repo, appidentity.Options{
		MinLength:     cfg.Security.PasswordMinLength,
		RequireUpper:  cfg.Security.PasswordUpper,
		RequireLower:  cfg.Security.PasswordLower,
		RequireDigit:  cfg.Security.PasswordDigit,
		RequireSymbol: cfg.Security.PasswordSymbol,
		History:       cfg.Security.PasswordHistory,
		MaxAgeDays:    cfg.Security.PasswordMaxAgeDay,
		SessionTTL:    cfg.Security.SessionTTL,
		MaxFailures:   cfg.Security.LoginMaxFailures,
		LockDuration:  cfg.Security.LoginLockDuration,
		Crypto:        crypt,
	})
	orgKey := env("VELORA_BOOTSTRAP_ORG_KEY", "default")
	orgName := env("VELORA_BOOTSTRAP_ORG_NAME", cfg.App.Name)
	admin := strings.TrimSpace(os.Getenv("VELORA_BOOTSTRAP_ADMIN"))
	adminPass := secret("VELORA_BOOTSTRAP_PASSWORD")
	if admin != "" && adminPass == "" {
		return nil, fmt.Errorf("VELORA_BOOTSTRAP_PASSWORD is required when VELORA_BOOTSTRAP_ADMIN is set")
	}
	if err = identitySvc.Bootstrap(ctx, orgKey, orgName, admin, adminPass); err != nil {
		return nil, fmt.Errorf("identity bootstrap: %w", err)
	}
	if admin == "" {
		log.Warn("no bootstrap administrator configured", "hint", "set VELORA_BOOTSTRAP_ADMIN and VELORA_BOOTSTRAP_PASSWORD")
	}

	auditWriter := audit.NewWriter(db)
	if hasTopic(cfg.Messaging.RocketMQTopics, "audit-events") {
		forwarder, forwarderErr := auditsink.NewReliableForwarder("audit-events")
		if forwarderErr != nil {
			return nil, forwarderErr
		}
		auditWriter = audit.NewWriterWithForwarder(db, forwarder)
	} else if cfg.Messaging.Provider == "rocketmq" {
		log.Warn("audit reliable forwarding disabled because audit-events is not in the RocketMQ topic allowlist")
	}
	var met *metrics.Metrics
	if cfg.Observability.MetricsEnabled {
		met = metrics.New()
	}

	tlsCfg, err := serverTLSConfig(cfg.Server)
	if err != nil {
		return nil, err
	}
	publicOperation := func(_ context.Context, operation string) bool {
		switch operation {
		case forgev1.OperationSystemServiceHealth, forgev1.OperationSystemServiceReadiness, forgev1.OperationIdentityServiceLogin, forgev1.OperationIdentityServiceBeginOIDCLogin, forgev1.OperationIdentityServiceCompleteOIDCLogin, forgev1.OperationIdentityServiceCompleteWeChatLogin, forgev1.OperationIdentityServiceLoginLDAP, forgev1.OperationPortalServiceConsumeApplicationEnrollment, forgev1.OperationPortalServiceGetApplicationDirectoryOrganization, forgev1.OperationPortalServiceListApplicationDirectoryDepartments, forgev1.OperationPortalServiceListApplicationDirectoryUsers:
			return false
		default:
			return true
		}
	}
	grpcSecurity := selector.Server(authn.Server(identitySvc), authz.Server(authz.Rules())).Match(publicOperation).Build()
	httpSecurity := selector.Server(authn.Server(identitySvc), csrf.Server(), authz.Server(authz.Rules())).Match(publicOperation).Build()
	httpOpts := []khttp.ServerOption{
		khttp.Address(cfg.Server.ListenAddr), khttp.Timeout(cfg.Server.WriteTimeout), khttp.Middleware(httpSecurity),
		khttp.ResponseEncoder(httpserver.EncodeResponse), khttp.ErrorEncoder(httpserver.EncodeError),
		khttp.Filter(httpserver.Filters(httpserver.FilterOptions{
			Log: log, Metrics: met, Secure: cfg.Security.SecureCookies,
			AllowedOrigins: cfg.Security.AllowedOrigins, MaxBodyBytes: cfg.Server.MaxBodyBytes, ServiceName: cfg.App.Name,
			TrustedProxies: cfg.Security.TrustedProxies,
		})...),
	}
	grpcOpts := []kgrpc.ServerOption{kgrpc.Address(cfg.Server.GRPCListenAddr), kgrpc.Timeout(cfg.Server.WriteTimeout), kgrpc.Middleware(ktracing.Server(), grpcSecurity)}
	if tlsCfg != nil {
		httpOpts = append(httpOpts, khttp.TLSConfig(tlsCfg.Clone()))
		grpcOpts = append(grpcOpts, kgrpc.TLSConfig(tlsCfg.Clone()))
	}
	httpServer := khttp.NewServer(httpOpts...)
	httpServer.ReadHeaderTimeout = 10 * time.Second
	httpServer.ReadTimeout = cfg.Server.ReadTimeout
	httpServer.WriteTimeout = cfg.Server.WriteTimeout
	httpServer.IdleTimeout = cfg.Server.IdleTimeout

	checks := []health.Check{
		{Name: "database", Provider: cfg.Database.Provider, Ping: db.PingContext},
		{Name: "cache", Provider: c.Provider(), Ping: c.Ping},
		{Name: "messaging", Provider: bus.Provider(), Ping: bus.Ping},
		{Name: "search", Provider: se.Provider(), Ping: se.Ping},
		{Name: "storage", Provider: st.Provider(), Ping: st.Ping},
	}
	providers := map[string]string{
		"database": cfg.Database.Provider, "cache": c.Provider(), "messaging": bus.Provider(),
		"search": se.Provider(), "storage": st.Provider(), "crypto": crypt.Name(),
		"discovery": reg.Provider(), "remote_config": cfg.RemoteConfig.Provider,
	}
	systemService := kratosapi.NewSystemService(cfg, opts.Version, checks, providers)
	platformService := kratosapi.NewPlatformService(identitySvc, portalSvc, approvalSvc, configChangeSvc, dataPolicySvc, auditWriter, db)
	portalService := kratosapi.NewPortalService(portalSvc, auditWriter, db)
	portalService.ConfigureIdempotency(idempotency.New(db))
	if c.Provider() != "disabled" {
		handoffStore, handoffErr := credentialhandoff.New(c, provisioningCipher)
		if handoffErr != nil {
			return nil, fmt.Errorf("credential handoff: %w", handoffErr)
		}
		portalService.ConfigureCredentialHandoff(handoffStore)
	}
	provisioningRouter, err := provisioninghttp.NewRouter(db, provisioningCipher, nil)
	if err != nil {
		return nil, fmt.Errorf("provisioning checks: %w", err)
	}
	portalService.ConfigureProvisioningRouter(provisioningRouter)
	portalService.ConfigureIdentityBoundary(cfg.Security.CasdoorAdminURL, cfg.Security.OIDCIssuer, cfg.Security.OIDCInternalURL, cfg.Security.CasdoorAllowedHosts, cfg.Security.ApplicationOnboardingV2, cfg.Security.CasdoorAdminEntryEnabled)
	casdoorAutomation, err := casdooradmin.New(casdooradmin.Config{BaseURL: cfg.Security.CasdoorAutomationURL, Token: cfg.Security.CasdoorAutomationToken, Owner: cfg.Security.CasdoorApplicationOwner, Organization: cfg.Security.CasdoorOrganization, Enabled: cfg.Security.CasdoorApplicationAutomationEnabled})
	if err != nil {
		return nil, fmt.Errorf("casdoor application automation: %w", err)
	}
	portalService.ConfigureCasdoorAutomation(casdoorAutomation)
	casdoorIdentity, err := casdooridentity.New(casdooridentity.Config{
		BaseURL: cfg.Security.OIDCInternalURL, ClientID: cfg.Security.CasdoorIdentityClientID,
		ClientSecret: cfg.Security.CasdoorIdentityClientSecret, Organization: cfg.Security.CasdoorOrganization,
		Application: cfg.Security.CasdoorApplication, Enabled: cfg.Security.CasdoorIdentityManagementEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("casdoor identity management: %w", err)
	}
	identitySvc.ConfigureManagedIdentityProvider(casdoorIdentity, cfg.Security.OIDCIssuer)
	identityService := kratosapi.NewIdentityService(identitySvc, auditWriter, db, ratelimit.New(c), cfg.Security.SecureCookies, cfg.Security.SameSite)
	identityService.ConfigureAuthMode(cfg.Security.AuthMode)
	if cfg.Security.TurnstileConfigured() {
		verifier, err := turnstile.New(turnstile.Config{
			Secret: cfg.Security.TurnstileSecret, Action: cfg.Security.EffectiveTurnstileAction(), Hostnames: cfg.Security.TurnstileHostnames,
		})
		if err != nil {
			return nil, fmt.Errorf("turnstile: %w", err)
		}
		identityService.ConfigureTurnstile(verifier)
		identityService.ConfigureLoginChallengeCache(c)
	}
	if cfg.Security.CasdoorPasswordLoginEnabled {
		providerName := strings.ToLower(strings.TrimSpace(cfg.Security.OIDCName))
		if providerName == "" {
			providerName = "casdoor"
		}
		identityService.ConfigureCasdoorPasswordLogin(true, oidcProviders[providerName])
		bridge, bridgeErr := kratosapi.NewSessionBridge(c, db, cfg.Security.CasdoorAccountURL, cfg.Server.PublicURL, cfg.Security.SecureCookies, kratosapi.SameSiteMode(cfg.Security.SameSite))
		if bridgeErr != nil {
			return nil, fmt.Errorf("casdoor session bridge: %w", bridgeErr)
		}
		bridge.ConfigureAccessControl(identitySvc.AuthenticateSessionID, func(ctx context.Context, principal domain.Principal, applicationID string) error {
			_, err := portalSvc.GetApplication(ctx, principal, applicationID)
			return err
		})
		identityService.ConfigureSessionBridge(bridge)
		wechatBroker, wechatErr := kratosapi.NewWeChatBroker(kratosapi.WeChatConfig{Enabled: cfg.Security.WeChatLoginEnabled, AppID: cfg.Security.WeChatAppID, Provider: cfg.Security.WeChatProvider, CallbackURL: cfg.Security.WeChatCallbackURL, Secure: cfg.Security.SecureCookies}, c, db, oidcProviders[providerName], bridge, casdoorIdentity, identityService)
		if wechatErr != nil {
			return nil, fmt.Errorf("WeChat login: %w", wechatErr)
		}
		if wechatBroker != nil {
			identityService.ConfigureWeChat(wechatBroker)
			httpServer.HandlePrefix("/_velora/wechat/", wechatBroker.Handler())
		} else {
			// Keep disabled identity endpoints fail-closed instead of falling
			// through to the SPA handler on the protocol-only auth host.
			httpServer.HandlePrefix("/_velora/wechat/", http.NotFoundHandler())
		}
		httpServer.Handle("/_velora/session/bridge", bridge.Handler())
		httpServer.Handle("/_velora/authorize", bridge.AuthorizationHandler())
		httpServer.Handle("/_velora/session/logout", bridge.GatewayLogoutHandler())
	}
	identityService.ConfigureFederatedLogin(kratosapi.FederatedLoginOptions{Cache: c, OIDC: oidcProviders, LDAP: ldapProviders})
	approvalService := kratosapi.NewApprovalService(approvalSvc, auditWriter, db)
	portalService.ConfigureApproval(approvalSvc)
	forgev1.RegisterSystemServiceHTTPServer(httpServer, systemService)
	forgev1.RegisterIdentityServiceHTTPServer(httpServer, identityService)
	forgev1.RegisterPlatformServiceHTTPServer(httpServer, platformService)
	forgev1.RegisterApprovalServiceHTTPServer(httpServer, approvalService)
	forgev1.RegisterPortalServiceHTTPServer(httpServer, portalService)
	if met != nil {
		httpServer.Handle(cfg.Observability.MetricsPath, met.Handler())
	}
	if !cfg.Compliance.DisableDebugEndpoints {
		httpServer.HandlePrefix("/debug/pprof/", httpserver.DebugHandler())
	}
	httpServer.HandlePrefix("/", httpserver.SPA(httpserver.SPAOptions{
		Root:            cfg.Server.WebDir,
		FrameSources:    cfg.Server.WebCSPFrameSources,
		ConnectSources:  cfg.Server.WebCSPConnectSources,
		WujieCSPEnabled: cfg.Server.WebCSPWujieEnabled,
	}))
	grpcServer := kgrpc.NewServer(grpcOpts...)
	forgev1.RegisterSystemServiceServer(grpcServer, systemService)
	forgev1.RegisterPlatformServiceServer(grpcServer, platformService)
	forgev1.RegisterIdentityServiceServer(grpcServer, identityService)
	forgev1.RegisterApprovalServiceServer(grpcServer, approvalService)
	forgev1.RegisterPortalServiceServer(grpcServer, portalService)
	runtime := kratos.New(
		kratos.Context(ctx), kratos.Name(cfg.App.Name), kratos.Version(opts.Version),
		kratos.Metadata(map[string]string{"environment": cfg.App.Environment, "region": cfg.App.Region, "zone": cfg.App.Zone}),
		kratos.Server(httpServer, grpcServer), kratos.StopTimeout(cfg.Server.ShutdownTimeout),
	)
	return &App{cfg: cfg, log: log, runtime: runtime, db: db, cache: c, bus: bus, registry: reg, traceShutdown: traceShutdown, portal: portalService}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.registry.Register(ctx); err != nil {
		return fmt.Errorf("service register: %w", err)
	}
	defer func() { _ = a.registry.Deregister(context.Background()) }()
	if a.portal != nil {
		go a.portal.RunProviderReconciler(ctx, 5*time.Minute)
	}
	a.log.Info("Kratos servers starting",
		"http_addr", a.cfg.Server.ListenAddr, "grpc_addr", a.cfg.Server.GRPCListenAddr,
		"public_url", a.cfg.Server.PublicURL, "database", a.cfg.Database.Provider,
		"cache", a.cache.Provider(), "messaging", a.bus.Provider(),
		"discovery", a.registry.Provider(), "tls", a.cfg.Server.TLSEnabled)
	return a.runtime.Run()
}
func (a *App) Close() {
	if a.runtime != nil {
		_ = a.runtime.Stop()
	}
	if a.registry != nil {
		_ = a.registry.Deregister(context.Background())
	}
	if a.traceShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = a.traceShutdown(ctx)
		cancel()
	}
	if a.bus != nil {
		a.bus.Close()
	}
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}

func serverTLSConfig(cfg config.Server) (*tls.Config, error) {
	if !cfg.TLSEnabled {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}
	t := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate}}
	if cfg.RequireClientTLS {
		raw, err := os.ReadFile(cfg.TLSClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read client CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(raw) {
			return nil, fmt.Errorf("invalid client CA PEM")
		}
		t.ClientCAs = pool
		t.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return t, nil
}

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func secret(k string) string {
	if p := strings.TrimSpace(os.Getenv(k + "_FILE")); p != "" {
		if b, e := securefile.Read(p); e == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return strings.TrimSpace(os.Getenv(k))
}
