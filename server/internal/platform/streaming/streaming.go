// Package streaming provides append-only event-stream integration. It is
// deliberately separate from business messaging: RocketMQ owns reliable
// commands/events, while Kafka is opt-in for analytics and stream processing.
package streaming

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/tlsx"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	maxRecordBytes     = 16 << 20
	maxHeaderCount     = 64
	maxHeaderBytes     = 32 << 10
	maxHeaderNameBytes = 128
)

type Record struct {
	Stream    string
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Timestamp time.Time
}

type Producer interface {
	Append(context.Context, Record) error
	Ping(context.Context) error
	Close()
	Provider() string
}

func New(cfg config.Streaming) (Producer, error) {
	switch cfg.Provider {
	case "disabled", "":
		return disabled{}, nil
	case "kafka":
		opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...), kgo.ClientID(cfg.ClientID)}
		tlsConfig, err := tlsx.ClientConfig(tlsx.ClientOptions{
			Enabled: cfg.TLS, CAFile: cfg.TLSCAFile, CertFile: cfg.TLSCertFile,
			KeyFile: cfg.TLSKeyFile, ServerName: cfg.TLSServerName,
		})
		if err != nil {
			return nil, fmt.Errorf("kafka streaming tls: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, kgo.DialTLSConfig(tlsConfig))
		}
		if cfg.Username != "" {
			opts = append(opts, kgo.SASL(plain.Auth{User: cfg.Username, Pass: cfg.Password}.AsMechanism()))
		}
		client, err := kgo.NewClient(opts...)
		if err != nil {
			return nil, err
		}
		return &kafka{client: client}, nil
	default:
		return nil, fmt.Errorf("unsupported streaming provider %q", cfg.Provider)
	}
}

type disabled struct{}

func (disabled) Append(context.Context, Record) error {
	return errors.New("streaming provider is disabled")
}
func (disabled) Ping(context.Context) error { return nil }
func (disabled) Close()                     {}
func (disabled) Provider() string           { return "disabled" }

type kafka struct {
	client *kgo.Client
}

func (k *kafka) Append(ctx context.Context, record Record) error {
	record, headers, err := prepareRecord(ctx, record)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	kafkaHeaders := make([]kgo.RecordHeader, 0, len(headers))
	for _, name := range names {
		kafkaHeaders = append(kafkaHeaders, kgo.RecordHeader{Key: name, Value: []byte(headers[name])})
	}
	if err := k.client.ProduceSync(ctx, &kgo.Record{
		Topic: record.Stream, Key: record.Key, Value: record.Value, Headers: kafkaHeaders, Timestamp: record.Timestamp,
	}).FirstErr(); err != nil {
		return fmt.Errorf("append kafka stream %q: %w", record.Stream, err)
	}
	return nil
}

func (k *kafka) Ping(ctx context.Context) error { return k.client.Ping(ctx) }
func (k *kafka) Close()                         { k.client.Close() }
func (k *kafka) Provider() string               { return "kafka" }

func prepareRecord(ctx context.Context, record Record) (Record, map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, nil, err
	}
	record.Stream = strings.TrimSpace(record.Stream)
	if record.Stream == "" || len(record.Stream) > 249 {
		return Record{}, nil, errors.New("streaming: valid stream name is required")
	}
	if len(record.Key)+len(record.Value) > maxRecordBytes {
		return Record{}, nil, fmt.Errorf("streaming: record exceeds %d bytes", maxRecordBytes)
	}
	if len(record.Headers) > maxHeaderCount {
		return Record{}, nil, fmt.Errorf("streaming: more than %d headers", maxHeaderCount)
	}
	headers := make(map[string]string, len(record.Headers))
	total := 0
	for rawName, value := range record.Headers {
		name := strings.ToLower(strings.TrimSpace(rawName))
		if !validHeaderName(name) || !utf8.ValidString(value) {
			return Record{}, nil, fmt.Errorf("streaming: invalid header %q", rawName)
		}
		if _, exists := headers[name]; exists {
			return Record{}, nil, fmt.Errorf("streaming: duplicate normalized header %q", name)
		}
		total += len(name) + len(value)
		if total > maxHeaderBytes {
			return Record{}, nil, fmt.Errorf("streaming: headers exceed %d bytes", maxHeaderBytes)
		}
		headers[name] = value
	}
	if headers["traceparent"] == "" {
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	}
	record.Key = append([]byte(nil), record.Key...)
	record.Value = append([]byte(nil), record.Value...)
	return record, headers, nil
}

func validHeaderName(name string) bool {
	if name == "" || len(name) > maxHeaderNameBytes {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}
