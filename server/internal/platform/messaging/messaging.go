package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

type Bus interface {
	Publish(context.Context, Message) (Receipt, error)
	Ping(context.Context) error
	Close()
	Provider() string
}

func New(cfg config.Messaging) (Bus, error) {
	switch cfg.Provider {
	case "disabled", "":
		return noop{}, nil
	case "rocketmq":
		return newRocketMQ(cfg)
	default:
		return nil, fmt.Errorf("unsupported messaging provider %q", cfg.Provider)
	}
}

type noop struct{}

func (noop) Publish(context.Context, Message) (Receipt, error) {
	return Receipt{}, errors.New("messaging provider is disabled")
}
func (noop) Ping(context.Context) error { return nil }
func (noop) Close()                     {}
func (noop) Provider() string           { return "disabled" }
