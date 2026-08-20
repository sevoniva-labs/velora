package messaging

import (
	"context"
	"fmt"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

// Delivery retains a private acknowledgement token so application code cannot
// acknowledge an arbitrary provider message. DecodeError marks poison input;
// such a delivery must be retried until the broker's DLQ policy takes effect.
type Delivery struct {
	Message           Message
	ProviderMessageID string
	DeliveryAttempt   int32
	DecodeError       error
	ackToken          any
}

type Consumer interface {
	Receive(context.Context) ([]Delivery, error)
	Ack(context.Context, Delivery) error
	Retry(context.Context, Delivery, time.Duration) error
	Close()
	Provider() string
	Group() string
}

func NewConsumer(cfg config.Messaging) (Consumer, error) {
	switch cfg.Provider {
	case "rocketmq":
		return newRocketMQConsumer(cfg)
	case "disabled", "":
		return nil, fmt.Errorf("messaging consumer is disabled")
	default:
		return nil, fmt.Errorf("unsupported messaging consumer provider %q", cfg.Provider)
	}
}
