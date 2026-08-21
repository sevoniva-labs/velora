// Command example-settlement-service demonstrates how a modular domain can be
// deployed as an independently secured Kratos HTTP/gRPC process.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2"
	ktracing "github.com/go-kratos/kratos/v2/middleware/tracing"
	kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	velorav1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/examples/settlement/application"
	"github.com/sevoniva-labs/velora/server/examples/settlement/domain"
	"github.com/sevoniva-labs/velora/server/examples/settlement/repository"
	settlementtransport "github.com/sevoniva-labs/velora/server/examples/settlement/transport"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/logx"
	"github.com/sevoniva-labs/velora/server/internal/platform/observability"
	"github.com/sevoniva-labs/velora/server/internal/platform/securefile"
	grpc_health "google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
)

const serviceName = "velora-example-settlement"

var version = "0.2.0-dev"

type runtimeConfig struct {
	HTTPAddress     string
	GRPCAddress     string
	OrganizationID  string
	TokenDigest     string
	TokenSubject    string
	TLSCertFile     string
	TLSKeyFile      string
	TLSClientCAFile string
	Observability   config.Observability
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("reference settlement service stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadRuntimeConfig()
	if err != nil {
		return err
	}
	logger := logx.New(cfg.Observability.LogLevel, cfg.Observability.LogFormat, serviceName, env("VELORA_ENV", "development"), version)
	slog.SetDefault(logger)
	traceShutdown, err := observability.InitTracing(ctx, cfg.Observability, serviceName, version, env("VELORA_ENV", "development"))
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := traceShutdown(shutdownCtx); err != nil {
			logger.Error("trace shutdown failed", "error", err)
		}
	}()

	store, err := repository.NewMemory(domain.Settlement{
		ID: "demo-settlement", OrganizationID: cfg.OrganizationID, Status: domain.StatusSettled,
		Currency: "CNY", AmountMinor: 10000, Version: 1, UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	query, err := application.NewQueryService(store)
	if err != nil {
		return err
	}
	service, err := settlementtransport.NewService(query)
	if err != nil {
		return err
	}
	authenticator, err := settlementtransport.NewTokenAuthenticator(cfg.TokenDigest, cfg.TokenSubject, cfg.OrganizationID)
	if err != nil {
		return err
	}
	tlsConfig, err := loadServerTLS(cfg)
	if err != nil {
		return err
	}

	httpOptions := []khttp.ServerOption{
		khttp.Address(cfg.HTTPAddress), khttp.Timeout(15 * time.Second),
		khttp.Middleware(ktracing.Server(), authn.Server(authenticator)),
	}
	grpcOptions := []kgrpc.ServerOption{
		kgrpc.Address(cfg.GRPCAddress), kgrpc.Timeout(15 * time.Second),
		kgrpc.Middleware(ktracing.Server(), authn.Server(authenticator)),
	}
	if tlsConfig != nil {
		httpOptions = append(httpOptions, khttp.TLSConfig(tlsConfig.Clone()))
		grpcOptions = append(grpcOptions, kgrpc.TLSConfig(tlsConfig.Clone()))
	}
	httpServer := khttp.NewServer(httpOptions...)
	httpServer.ReadHeaderTimeout = 10 * time.Second
	httpServer.ReadTimeout = 15 * time.Second
	httpServer.WriteTimeout = 15 * time.Second
	httpServer.IdleTimeout = 60 * time.Second
	httpServer.HandleFunc("/healthz", healthHandler)
	httpServer.HandleFunc("/readyz", healthHandler)
	velorav1.RegisterReferenceSettlementServiceHTTPServer(httpServer, service)

	grpcServer := kgrpc.NewServer(grpcOptions...)
	velorav1.RegisterReferenceSettlementServiceServer(grpcServer, service)
	grpcHealth := grpc_health.NewServer()
	grpcHealth.SetServingStatus(serviceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, grpcHealth)

	app := kratos.New(
		kratos.Context(ctx), kratos.Name(serviceName), kratos.Version(version),
		kratos.Metadata(map[string]string{"domain": "example-settlement", "deployment": "independent"}),
		kratos.Server(httpServer, grpcServer), kratos.StopTimeout(15*time.Second),
	)
	logger.Info("reference settlement service starting", "http_addr", cfg.HTTPAddress, "grpc_addr", cfg.GRPCAddress, "mtls", tlsConfig != nil)
	return app.Run()
}

func loadRuntimeConfig() (runtimeConfig, error) {
	tracingEnabled, err := boolEnv("VELORA_TRACING_ENABLED", false)
	if err != nil {
		return runtimeConfig{}, err
	}
	otlpTLS, err := boolEnv("VELORA_OTLP_TLS", true)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg := runtimeConfig{
		HTTPAddress:     env("VELORA_EXAMPLE_SETTLEMENT_HTTP_ADDR", "127.0.0.1:18080"),
		GRPCAddress:     env("VELORA_EXAMPLE_SETTLEMENT_GRPC_ADDR", "127.0.0.1:19090"),
		OrganizationID:  strings.TrimSpace(os.Getenv("VELORA_EXAMPLE_SETTLEMENT_ORGANIZATION_ID")),
		TokenDigest:     secret("VELORA_EXAMPLE_SETTLEMENT_TOKEN_SHA256"),
		TokenSubject:    env("VELORA_EXAMPLE_SETTLEMENT_TOKEN_SUBJECT", "reference-settlement-client"),
		TLSCertFile:     strings.TrimSpace(os.Getenv("VELORA_EXAMPLE_SETTLEMENT_TLS_CERT_FILE")),
		TLSKeyFile:      strings.TrimSpace(os.Getenv("VELORA_EXAMPLE_SETTLEMENT_TLS_KEY_FILE")),
		TLSClientCAFile: strings.TrimSpace(os.Getenv("VELORA_EXAMPLE_SETTLEMENT_TLS_CLIENT_CA_FILE")),
		Observability: config.Observability{
			LogLevel: env("VELORA_LOG_LEVEL", "info"), LogFormat: env("VELORA_LOG_FORMAT", "json"),
			TracingEnabled: tracingEnabled, OTLPEndpoint: strings.TrimSpace(os.Getenv("VELORA_OTLP_ENDPOINT")),
			OTLPTLS: otlpTLS, OTLPTLSCAFile: strings.TrimSpace(os.Getenv("VELORA_OTLP_TLS_CA_FILE")),
			OTLPTLSCertFile:   strings.TrimSpace(os.Getenv("VELORA_OTLP_TLS_CERT_FILE")),
			OTLPTLSKeyFile:    strings.TrimSpace(os.Getenv("VELORA_OTLP_TLS_KEY_FILE")),
			OTLPTLSServerName: strings.TrimSpace(os.Getenv("VELORA_OTLP_TLS_SERVER_NAME")),
		},
	}
	if cfg.OrganizationID == "" {
		return runtimeConfig{}, errors.New("VELORA_EXAMPLE_SETTLEMENT_ORGANIZATION_ID is required")
	}
	if cfg.TokenDigest == "" {
		return runtimeConfig{}, errors.New("VELORA_EXAMPLE_SETTLEMENT_TOKEN_SHA256 or its _FILE variant is required")
	}
	return cfg, nil
}

func loadServerTLS(cfg runtimeConfig) (*tls.Config, error) {
	files := []string{cfg.TLSCertFile, cfg.TLSKeyFile, cfg.TLSClientCAFile}
	configured := 0
	for _, file := range files {
		if file != "" {
			configured++
		}
	}
	if configured == 0 {
		if loopbackAddress(cfg.HTTPAddress) && loopbackAddress(cfg.GRPCAddress) {
			return nil, nil
		}
		return nil, errors.New("non-loopback reference service listeners require mTLS")
	}
	if configured != len(files) {
		return nil, errors.New("mTLS requires certificate, private key, and client CA files")
	}
	certificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load reference service certificate: %w", err)
	}
	rawCA, err := securefile.Read(cfg.TLSClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read reference service client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(rawCA) {
		return nil, errors.New("reference service client CA is not valid PEM")
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate},
		ClientCAs: clientCAs, ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}

func loopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func healthHandler(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	if _, err := response.Write([]byte("{\"status\":\"UP\"}")); err != nil {
		slog.Debug("health response disconnected", "error", err)
	}
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func boolEnv(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func secret(name string) string {
	if path := strings.TrimSpace(os.Getenv(name + "_FILE")); path != "" {
		raw, err := securefile.Read(path)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(raw))
	}
	return strings.TrimSpace(os.Getenv(name))
}
