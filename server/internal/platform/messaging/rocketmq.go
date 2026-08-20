package messaging

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/tlsx"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	rocketMQMaxSendBytes = 4 << 20
	rocketMQMaxRecvBytes = 16 << 20
)

type rocketMQProducer interface {
	Send(context.Context, *rmq.Message) ([]*rmq.SendReceipt, error)
	Start() error
	GracefulStop() error
}

type rocketMQProducerFactory func(config.Messaging, *tls.Config, []string) (rocketMQProducer, error)

type rocketMQ struct {
	producer rocketMQProducer
	topics   map[string]struct{}
	closed   atomic.Bool
}

func newRocketMQ(cfg config.Messaging) (Bus, error) {
	return newRocketMQWithFactory(cfg, newApacheRocketMQProducer)
}

func newRocketMQWithFactory(cfg config.Messaging, factory rocketMQProducerFactory) (*rocketMQ, error) {
	topics, allowed, err := rocketMQTopics(cfg.RocketMQTopics)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := rocketMQTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("rocketmq tls: %w", err)
	}
	producer, err := factory(cfg, tlsConfig, topics)
	if err != nil {
		return nil, fmt.Errorf("create rocketmq producer: %w", err)
	}
	if err := producer.Start(); err != nil {
		_ = producer.GracefulStop()
		return nil, fmt.Errorf("start rocketmq producer: %w", err)
	}
	return &rocketMQ{producer: producer, topics: allowed}, nil
}

func newApacheRocketMQProducer(cfg config.Messaging, tlsConfig *tls.Config, topics []string) (rocketMQProducer, error) {
	return rmq.NewProducer(&rmq.Config{
		Endpoint:      strings.TrimSpace(cfg.RocketMQEndpoint),
		NameSpace:     strings.TrimSpace(cfg.RocketMQNamespace),
		ConsumerGroup: strings.TrimSpace(cfg.RocketMQGroup),
		Credentials: &credentials.SessionCredentials{
			AccessKey:    cfg.RocketMQAccessKey,
			AccessSecret: cfg.RocketMQSecretKey,
		},
	}, rmq.WithTopics(topics...), rmq.WithClientFunc(rocketMQClientFactory(tlsConfig)))
}

func rocketMQClientFactory(tlsConfig *tls.Config) rmq.NewClientFunc {
	return func(rmqConfig *rmq.Config, _ ...rmq.ClientOption) (rmq.Client, error) {
		return rmq.NewClient(rmqConfig, rmq.WithClientConnFunc(rocketMQConnFactory(tlsConfig)))
	}
}

func rocketMQTLSConfig(cfg config.Messaging) (*tls.Config, error) {
	return tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled: cfg.TLS, CAFile: cfg.TLSCAFile, CertFile: cfg.TLSCertFile,
		KeyFile: cfg.TLSKeyFile, ServerName: cfg.TLSServerName,
	})
}

// rocketMQConnFactory replaces the SDK's insecure default TLS configuration.
// Plaintext is allowed only when the scaffold configuration explicitly disables
// TLS; TLS mode always verifies the certificate chain and hostname.
func rocketMQConnFactory(tlsConfig *tls.Config) rmq.ClientConnFunc {
	return func(endpoint string, _ ...rmq.ConnOption) (rmq.ClientConn, error) {
		var transportCredentials grpccredentials.TransportCredentials
		if tlsConfig == nil {
			transportCredentials = insecure.NewCredentials()
		} else {
			transportCredentials = grpccredentials.NewTLS(tlsConfig.Clone())
		}
		conn, err := grpc.NewClient(endpoint,
			grpc.WithTransportCredentials(transportCredentials),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallSendMsgSize(rocketMQMaxSendBytes),
				grpc.MaxCallRecvMsgSize(rocketMQMaxRecvBytes),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("rocketmq grpc client %q: %w", endpoint, err)
		}
		return &rocketMQClientConn{conn: conn}, nil
	}
}

type rocketMQClientConn struct {
	conn *grpc.ClientConn
}

func (c *rocketMQClientConn) Conn() *grpc.ClientConn { return c.conn }
func (c *rocketMQClientConn) Close() error           { return c.conn.Close() }

func (r *rocketMQ) Publish(ctx context.Context, message Message) (Receipt, error) {
	if r.closed.Load() {
		return Receipt{}, errors.New("rocketmq producer is closed")
	}
	message, rmqMessage, err := rocketMQPublishingMessage(ctx, message)
	if err != nil {
		return Receipt{}, err
	}
	if _, ok := r.topics[message.Topic]; !ok {
		return Receipt{}, fmt.Errorf("rocketmq topic %q is not configured", message.Topic)
	}
	receipts, err := r.producer.Send(ctx, rmqMessage)
	if err != nil {
		return Receipt{}, fmt.Errorf("publish rocketmq topic %q: %w", message.Topic, err)
	}
	if len(receipts) == 0 {
		return Receipt{}, fmt.Errorf("publish rocketmq topic %q: empty send receipt", message.Topic)
	}
	return Receipt{Provider: "rocketmq", ProviderMessageID: receipts[0].MessageID}, nil
}

func rocketMQPublishingMessage(ctx context.Context, message Message) (Message, *rmq.Message, error) {
	message, headers, err := prepareMessage(ctx, message)
	if err != nil {
		return Message{}, nil, err
	}
	rmqMessage := &rmq.Message{Topic: message.Topic, Body: message.Body}
	keys := []string{message.ID}
	if len(message.Key) > 0 {
		if utf8.Valid(message.Key) {
			keys = append(keys, string(message.Key))
		} else {
			keys = append(keys, base64.RawURLEncoding.EncodeToString(message.Key))
			headers[HeaderKeyEncoding] = "base64url"
		}
	}
	rmqMessage.SetKeys(keys...)
	if message.Tag != "" {
		rmqMessage.SetTag(message.Tag)
	}
	if message.OrderingKey != "" {
		rmqMessage.SetMessageGroup(message.OrderingKey)
	}
	if !message.DeliverAt.IsZero() {
		rmqMessage.SetDelayTimestamp(message.DeliverAt)
	}
	for name, value := range headers {
		rmqMessage.AddProperty(name, value)
	}
	return message, rmqMessage, nil
}

// Start synchronizes producer settings and topic routes with the Proxy. The
// official SDK exposes no public heartbeat, so readiness means that startup
// succeeded and this producer has not been closed.
func (r *rocketMQ) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.closed.Load() {
		return errors.New("rocketmq producer is closed")
	}
	return nil
}

func (r *rocketMQ) Close() {
	if r.closed.CompareAndSwap(false, true) {
		_ = r.producer.GracefulStop()
	}
}

func (r *rocketMQ) Provider() string { return "rocketmq" }

func rocketMQTopics(values []string) ([]string, map[string]struct{}, error) {
	topics := make([]string, 0, len(values))
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		topic := strings.TrimSpace(value)
		if topic == "" {
			continue
		}
		if _, exists := allowed[topic]; exists {
			continue
		}
		allowed[topic] = struct{}{}
		topics = append(topics, topic)
	}
	if len(topics) == 0 {
		return nil, nil, errors.New("rocketmq requires at least one configured topic")
	}
	return topics, allowed, nil
}
