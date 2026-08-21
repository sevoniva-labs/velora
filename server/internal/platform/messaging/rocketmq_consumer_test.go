package messaging

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

type fakeRocketMQConsumerClient struct {
	sources    []rocketMQSourceDelivery
	started    int
	stopped    int
	acked      any
	retried    any
	retryDelay time.Duration
}

func (f *fakeRocketMQConsumerClient) Start() error        { f.started++; return nil }
func (f *fakeRocketMQConsumerClient) GracefulStop() error { f.stopped++; return nil }
func (f *fakeRocketMQConsumerClient) Receive(context.Context, int32, time.Duration) ([]rocketMQSourceDelivery, error) {
	return f.sources, nil
}
func (f *fakeRocketMQConsumerClient) Ack(_ context.Context, token any) error {
	f.acked = token
	return nil
}
func (f *fakeRocketMQConsumerClient) Retry(token any, delay time.Duration) error {
	f.retried, f.retryDelay = token, delay
	return nil
}

func TestRocketMQConsumerDecodesAcknowledgesAndRetries(t *testing.T) {
	fake := &fakeRocketMQConsumerClient{sources: []rocketMQSourceDelivery{{
		token: "receipt-1", topic: "business-events", providerMessageID: "rmq-1", body: []byte(`{"ok":true}`),
		properties: map[string]string{HeaderEventID: "event-1", HeaderEventType: "account.updated", HeaderOrganizationID: "org-1", "correlation-id": "request-1"},
		keys:       []string{"event-1", "account-1"}, orderingKey: "account-1", deliveryAttempt: 2,
	}}}
	cfg := config.Messaging{RocketMQGroup: "forge-consumer", RocketMQTopics: []string{"business-events"}}
	consumer, err := newRocketMQConsumerWithFactory(cfg, func(config.Messaging, *tls.Config, []string) (rocketMQConsumerClient, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQConsumerWithFactory() error = %v", err)
	}
	deliveries, err := consumer.Receive(context.Background())
	if err != nil || len(deliveries) != 1 {
		t.Fatalf("Receive() deliveries = %#v, error = %v", deliveries, err)
	}
	delivery := deliveries[0]
	if delivery.DecodeError != nil || delivery.Message.ID != "event-1" || string(delivery.Message.Key) != "account-1" {
		t.Fatalf("delivery = %#v", delivery)
	}
	if delivery.DeliveryAttempt != 2 || delivery.Message.Headers["correlation-id"] != "request-1" {
		t.Fatalf("delivery metadata = %#v", delivery)
	}
	if err := consumer.Ack(context.Background(), delivery); err != nil || fake.acked != "receipt-1" {
		t.Fatalf("Ack() error = %v, token = %#v", err, fake.acked)
	}
	if err := consumer.Retry(context.Background(), delivery, 3*time.Second); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if fake.retried != "receipt-1" || fake.retryDelay != 3*time.Second {
		t.Fatalf("retry = %#v at %v", fake.retried, fake.retryDelay)
	}
	consumer.Close()
	consumer.Close()
	if fake.stopped != 1 {
		t.Fatalf("GracefulStop() calls = %d", fake.stopped)
	}
}

func TestRocketMQConsumerRefusesToAckMalformedEnvelope(t *testing.T) {
	fake := &fakeRocketMQConsumerClient{sources: []rocketMQSourceDelivery{{
		token: "receipt-1", topic: "business-events", providerMessageID: "rmq-1", properties: map[string]string{},
	}}}
	consumer, err := newRocketMQConsumerWithFactory(config.Messaging{RocketMQGroup: "group", RocketMQTopics: []string{"business-events"}}, func(config.Messaging, *tls.Config, []string) (rocketMQConsumerClient, error) {
		return fake, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQConsumerWithFactory() error = %v", err)
	}
	defer consumer.Close()
	deliveries, err := consumer.Receive(context.Background())
	if err != nil || len(deliveries) != 1 || deliveries[0].DecodeError == nil {
		t.Fatalf("malformed delivery = %#v, error = %v", deliveries, err)
	}
	if err := consumer.Ack(context.Background(), deliveries[0]); err == nil {
		t.Fatal("Ack() accepted malformed delivery")
	}
}
