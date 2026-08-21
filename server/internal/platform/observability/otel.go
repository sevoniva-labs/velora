package observability

import (
	"context"
	"errors"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/tlsx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

type Shutdown func(context.Context) error

func InitTracing(ctx context.Context, cfg config.Observability, service, version, env string) (Shutdown, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if !cfg.TracingEnabled {
		return func(context.Context) error { return nil }, nil
	}
	if cfg.OTLPEndpoint == "" {
		return nil, errors.New("tracing enabled but otlp endpoint is empty")
	}
	tlsCfg, err := tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled: cfg.OTLPTLS, CAFile: cfg.OTLPTLSCAFile, CertFile: cfg.OTLPTLSCertFile,
		KeyFile: cfg.OTLPTLSKeyFile, ServerName: cfg.OTLPTLSServerName,
	})
	if err != nil {
		return nil, err
	}
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint)}
	if tlsCfg != nil {
		opts = append(opts, otlptracehttp.WithTLSClientConfig(tlsCfg))
	}
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	res, err := sdkresource.New(ctx, sdkresource.WithAttributes(semconv.ServiceName(service), semconv.ServiceVersion(version), attribute.String("deployment.environment.name", env)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
