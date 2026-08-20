package config

import (
	"strings"
	"testing"
)

func productionConfig() Config {
	cfg := Default()
	cfg.App.Environment = "production"
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=verify-full"
	return cfg
}

func TestValidateRequiresProviderTLSInProduction(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		prepare func(*Config)
	}{
		{name: "kafka", want: "streaming.tls", prepare: func(c *Config) {
			c.Streaming.Provider = "kafka"
			c.Streaming.Brokers = []string{"kafka:9092"}
		}},
		{name: "rocketmq", want: "messaging.tls", prepare: func(c *Config) {
			c.Messaging.Provider = "rocketmq"
			c.Messaging.RocketMQEndpoint = "rocketmq:8081"
		}},
		{name: "search flag", want: "search.tls", prepare: func(c *Config) {
			c.Search.Provider = "elasticsearch"
			c.Search.URLs = []string{"https://search:9200"}
		}},
		{name: "search scheme", want: "search.urls", prepare: func(c *Config) {
			c.Search.Provider = "opensearch"
			c.Search.URLs = []string{"http://search:9200"}
			c.Search.TLS = true
		}},
		{name: "s3 flag", want: "storage.tls", prepare: func(c *Config) {
			c.Storage.Provider = "oss"
			c.Storage.Bucket = "documents"
		}},
		{name: "s3 scheme", want: "storage.endpoint", prepare: func(c *Config) {
			c.Storage.Provider = "cos"
			c.Storage.Bucket = "documents"
			c.Storage.Endpoint = "http://cos.internal"
			c.Storage.TLS = true
		}},
		{name: "otlp flag", want: "observability.otlp_tls", prepare: func(c *Config) {
			c.Observability.TracingEnabled = true
			c.Observability.OTLPEndpoint = "https://otel:4318"
		}},
		{name: "otlp scheme", want: "observability.otlp_endpoint must use https", prepare: func(c *Config) {
			c.Observability.TracingEnabled = true
			c.Observability.OTLPEndpoint = "http://otel:4318"
			c.Observability.OTLPTLS = true
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := productionConfig()
			tt.prepare(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unsafe production provider should contain %q, got %v", tt.want, err)
			}
		})
	}
}

func TestValidateAllowsSecureProvidersInProduction(t *testing.T) {
	cfg := productionConfig()
	cfg.Streaming = Streaming{Provider: "kafka", Brokers: []string{"kafka:9093"}, TLS: true}
	cfg.Search = Search{Provider: "opensearch", URLs: []string{"https://search:9200"}, TLS: true}
	cfg.Storage = Storage{Provider: "ceph-rgw", Endpoint: "https://rgw.internal", Bucket: "documents", TLS: true}
	cfg.Observability.TracingEnabled = true
	cfg.Observability.OTLPEndpoint = "https://otel:4318"
	cfg.Observability.OTLPTLS = true

	if err := cfg.Validate(); err != nil {
		t.Fatalf("secure production providers rejected: %v", err)
	}
}

func TestValidateAllowsPlaintextProvidersInDevelopment(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	cfg.Streaming = Streaming{Provider: "kafka", Brokers: []string{"kafka:9092"}}
	cfg.Search = Search{Provider: "elasticsearch", URLs: []string{"http://search:9200"}}
	cfg.Storage = Storage{Provider: "s3-compatible", Endpoint: "http://s3:9000", Bucket: "documents"}
	cfg.Observability.TracingEnabled = true
	cfg.Observability.OTLPEndpoint = "http://otel:4318"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("development providers unexpectedly rejected: %v", err)
	}
}
