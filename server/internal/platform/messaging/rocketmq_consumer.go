package messaging

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

type rocketMQSourceDelivery struct {
	token             any
	topic             string
	providerMessageID string
	body              []byte
	properties        map[string]string
	keys              []string
	tag               string
	orderingKey       string
	deliverAt         time.Time
	deliveryAttempt   int32
}

type rocketMQConsumerClient interface {
	Start() error
	Receive(context.Context, int32, time.Duration) ([]rocketMQSourceDelivery, error)
	Ack(context.Context, any) error
	Retry(any, time.Duration) error
	GracefulStop() error
}

type rocketMQConsumerFactory func(config.Messaging, *tls.Config, []string) (rocketMQConsumerClient, error)

type rocketMQConsumer struct {
	client            rocketMQConsumerClient
	batchSize         int32
	invisibleDuration time.Duration
	group             string
	closed            atomic.Bool
}

func newRocketMQConsumer(cfg config.Messaging) (Consumer, error) {
	return newRocketMQConsumerWithFactory(cfg, newApacheRocketMQConsumer)
}

func newRocketMQConsumerWithFactory(cfg config.Messaging, factory rocketMQConsumerFactory) (*rocketMQConsumer, error) {
	if cfg.RocketMQGroup == "" {
		return nil, errors.New("rocketmq consumer group is required")
	}
	topics, _, err := rocketMQTopics(cfg.RocketMQTopics)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := rocketMQTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("rocketmq consumer tls: %w", err)
	}
	client, err := factory(cfg, tlsConfig, topics)
	if err != nil {
		return nil, fmt.Errorf("create rocketmq consumer: %w", err)
	}
	if err := client.Start(); err != nil {
		_ = client.GracefulStop()
		return nil, fmt.Errorf("start rocketmq consumer: %w", err)
	}
	batchSize := cfg.RocketMQBatchSize
	if batchSize <= 0 || batchSize > 64 {
		batchSize = 16
	}
	invisibleDuration := cfg.RocketMQInvisibleDuration
	if invisibleDuration < 10*time.Second || invisibleDuration > 12*time.Hour {
		invisibleDuration = 30 * time.Second
	}
	return &rocketMQConsumer{client: client, batchSize: int32(batchSize), invisibleDuration: invisibleDuration, group: cfg.RocketMQGroup}, nil
}

func newApacheRocketMQConsumer(cfg config.Messaging, tlsConfig *tls.Config, topics []string) (rocketMQConsumerClient, error) {
	subscriptions := make(map[string]*rmq.FilterExpression, len(topics))
	for _, topic := range topics {
		subscriptions[topic] = rmq.NewFilterExpression("*")
	}
	awaitDuration := cfg.RocketMQAwaitDuration
	if awaitDuration <= 0 || awaitDuration > 30*time.Second {
		awaitDuration = 5 * time.Second
	}
	client, err := rmq.NewSimpleConsumer(&rmq.Config{
		Endpoint: cfg.RocketMQEndpoint, NameSpace: cfg.RocketMQNamespace, ConsumerGroup: cfg.RocketMQGroup,
		Credentials: &credentials.SessionCredentials{AccessKey: cfg.RocketMQAccessKey, AccessSecret: cfg.RocketMQSecretKey},
	}, rmq.WithSimpleSubscriptionExpressions(subscriptions), rmq.WithSimpleAwaitDuration(awaitDuration),
		rmq.WithClientFuncForSimpleConsumer(rocketMQClientFactory(tlsConfig)))
	if err != nil {
		return nil, err
	}
	return &apacheRocketMQConsumer{client: client}, nil
}

type apacheRocketMQConsumer struct {
	client rmq.SimpleConsumer
}

func (a *apacheRocketMQConsumer) Start() error { return a.client.Start() }
func (a *apacheRocketMQConsumer) GracefulStop() error {
	return a.client.GracefulStop()
}
func (a *apacheRocketMQConsumer) Receive(ctx context.Context, batch int32, invisible time.Duration) ([]rocketMQSourceDelivery, error) {
	views, err := a.client.Receive(ctx, batch, invisible)
	if err != nil {
		return nil, err
	}
	deliveries := make([]rocketMQSourceDelivery, 0, len(views))
	for _, view := range views {
		source := rocketMQSourceDelivery{
			token: view, topic: view.GetTopic(), providerMessageID: view.GetMessageId(), body: append([]byte(nil), view.GetBody()...),
			properties: cloneStringMap(view.GetProperties()), keys: append([]string(nil), view.GetKeys()...),
			deliveryAttempt: view.GetDeliveryAttempt(),
		}
		if value := view.GetTag(); value != nil {
			source.tag = *value
		}
		if value := view.GetMessageGroup(); value != nil {
			source.orderingKey = *value
		}
		if value := view.GetDeliveryTimestamp(); value != nil {
			source.deliverAt = *value
		}
		deliveries = append(deliveries, source)
	}
	return deliveries, nil
}
func (a *apacheRocketMQConsumer) Ack(ctx context.Context, token any) error {
	view, ok := token.(*rmq.MessageView)
	if !ok || view == nil {
		return errors.New("invalid rocketmq acknowledgement token")
	}
	return a.client.Ack(ctx, view)
}
func (a *apacheRocketMQConsumer) Retry(token any, delay time.Duration) error {
	view, ok := token.(*rmq.MessageView)
	if !ok || view == nil {
		return errors.New("invalid rocketmq retry token")
	}
	return a.client.ChangeInvisibleDuration(view, delay)
}

func (r *rocketMQConsumer) Receive(ctx context.Context) ([]Delivery, error) {
	if r.closed.Load() {
		return nil, errors.New("rocketmq consumer is closed")
	}
	sources, err := r.client.Receive(ctx, r.batchSize, r.invisibleDuration)
	if err != nil {
		return nil, fmt.Errorf("receive rocketmq messages: %w", err)
	}
	deliveries := make([]Delivery, 0, len(sources))
	for _, source := range sources {
		message, decodeErr := decodeRocketMQDelivery(ctx, source)
		deliveries = append(deliveries, Delivery{
			Message: message, ProviderMessageID: source.providerMessageID,
			DeliveryAttempt: source.deliveryAttempt, DecodeError: decodeErr, ackToken: source.token,
		})
	}
	return deliveries, nil
}

func (r *rocketMQConsumer) Ack(ctx context.Context, delivery Delivery) error {
	if r.closed.Load() {
		return errors.New("rocketmq consumer is closed")
	}
	if delivery.DecodeError != nil {
		return errors.New("refusing to acknowledge malformed rocketmq delivery")
	}
	if delivery.ackToken == nil {
		return errors.New("missing rocketmq acknowledgement token")
	}
	if err := r.client.Ack(ctx, delivery.ackToken); err != nil {
		return fmt.Errorf("ack rocketmq message %q: %w", delivery.ProviderMessageID, err)
	}
	return nil
}

func (r *rocketMQConsumer) Retry(ctx context.Context, delivery Delivery, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.closed.Load() {
		return errors.New("rocketmq consumer is closed")
	}
	if delivery.ackToken == nil {
		return errors.New("missing rocketmq retry token")
	}
	if delay < time.Second || delay > 12*time.Hour {
		return errors.New("rocketmq retry delay must be between 1s and 12h")
	}
	if err := r.client.Retry(delivery.ackToken, delay); err != nil {
		return fmt.Errorf("retry rocketmq message %q: %w", delivery.ProviderMessageID, err)
	}
	return nil
}

func (r *rocketMQConsumer) Close() {
	if r.closed.CompareAndSwap(false, true) {
		_ = r.client.GracefulStop()
	}
}

func (r *rocketMQConsumer) Provider() string { return "rocketmq" }
func (r *rocketMQConsumer) Group() string    { return r.group }

func decodeRocketMQDelivery(ctx context.Context, source rocketMQSourceDelivery) (Message, error) {
	properties := cloneStringMap(source.properties)
	message := Message{
		ID: properties[HeaderEventID], OrganizationID: properties[HeaderOrganizationID], Topic: source.topic,
		Type: properties[HeaderEventType], Body: append([]byte(nil), source.body...),
		Headers: properties, Tag: source.tag, OrderingKey: source.orderingKey, DeliverAt: source.deliverAt,
	}
	delete(message.Headers, HeaderEventID)
	delete(message.Headers, HeaderEventType)
	delete(message.Headers, HeaderOrganizationID)
	keyEncoding := message.Headers[HeaderKeyEncoding]
	delete(message.Headers, HeaderKeyEncoding)
	if len(source.keys) > 1 {
		if keyEncoding == "base64url" {
			decoded, err := base64.RawURLEncoding.DecodeString(source.keys[1])
			if err != nil {
				return message, fmt.Errorf("decode rocketmq business key: %w", err)
			}
			message.Key = decoded
		} else {
			message.Key = []byte(source.keys[1])
		}
	}
	if _, _, err := prepareMessage(ctx, message); err != nil {
		return message, fmt.Errorf("decode rocketmq envelope: %w", err)
	}
	return message, nil
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
