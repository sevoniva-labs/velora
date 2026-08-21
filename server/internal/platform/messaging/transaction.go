package messaging

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type TransactionResolution uint8

const (
	TransactionUnknown TransactionResolution = iota
	TransactionCommit
	TransactionRollback
)

type TransactionChecker func(context.Context, Message) (TransactionResolution, error)
type LocalTransaction func(context.Context) error

type TransactionalPublisher interface {
	PublishTransaction(context.Context, Message, LocalTransaction) (Receipt, error)
	Close()
	Provider() string
}

func NewTransactionalPublisher(cfg config.Messaging, checker TransactionChecker) (TransactionalPublisher, error) {
	if cfg.Provider != "rocketmq" {
		return nil, fmt.Errorf("transactional business messages require rocketmq, got %q", cfg.Provider)
	}
	return newRocketMQTransactionalPublisher(cfg, checker)
}

type rocketMQTransactionProducer interface {
	Start() error
	GracefulStop() error
	BeginTransaction() rmq.Transaction
	SendWithTransaction(context.Context, *rmq.Message, rmq.Transaction) ([]*rmq.SendReceipt, error)
}

type rocketMQTransactionFactory func(config.Messaging, *tls.Config, []string, *rmq.TransactionChecker) (rocketMQTransactionProducer, error)

type rocketMQTransactionalPublisher struct {
	producer rocketMQTransactionProducer
	topics   map[string]struct{}
	closed   atomic.Bool
}

func newRocketMQTransactionalPublisher(cfg config.Messaging, checker TransactionChecker) (TransactionalPublisher, error) {
	return newRocketMQTransactionalPublisherWithFactory(cfg, checker, newApacheRocketMQTransactionProducer)
}

func newRocketMQTransactionalPublisherWithFactory(cfg config.Messaging, checker TransactionChecker, factory rocketMQTransactionFactory) (*rocketMQTransactionalPublisher, error) {
	if checker == nil {
		return nil, errors.New("rocketmq transaction checker is required")
	}
	topics, allowed, err := rocketMQTopics(cfg.RocketMQTopics)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := rocketMQTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("rocketmq transaction tls: %w", err)
	}
	sdkChecker := &rmq.TransactionChecker{Check: func(view *rmq.MessageView) rmq.TransactionResolution {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		message, err := decodeRocketMQTransactionCheck(ctx, view)
		if err != nil {
			return rmq.UNKNOWN
		}
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(message.Headers))
		resolution, err := checker(ctx, message)
		if err != nil {
			return rmq.UNKNOWN
		}
		return toRocketMQResolution(resolution)
	}}
	producer, err := factory(cfg, tlsConfig, topics, sdkChecker)
	if err != nil {
		return nil, fmt.Errorf("create rocketmq transaction producer: %w", err)
	}
	if err := producer.Start(); err != nil {
		_ = producer.GracefulStop()
		return nil, fmt.Errorf("start rocketmq transaction producer: %w", err)
	}
	return &rocketMQTransactionalPublisher{producer: producer, topics: allowed}, nil
}

func newApacheRocketMQTransactionProducer(cfg config.Messaging, tlsConfig *tls.Config, topics []string, checker *rmq.TransactionChecker) (rocketMQTransactionProducer, error) {
	return rmq.NewProducer(&rmq.Config{
		Endpoint: cfg.RocketMQEndpoint, NameSpace: cfg.RocketMQNamespace,
		Credentials: &credentials.SessionCredentials{AccessKey: cfg.RocketMQAccessKey, AccessSecret: cfg.RocketMQSecretKey},
	}, rmq.WithTopics(topics...), rmq.WithTransactionChecker(checker), rmq.WithClientFunc(rocketMQClientFactory(tlsConfig)))
}

func (p *rocketMQTransactionalPublisher) PublishTransaction(ctx context.Context, message Message, local LocalTransaction) (Receipt, error) {
	if p.closed.Load() {
		return Receipt{}, errors.New("rocketmq transaction producer is closed")
	}
	if local == nil {
		return Receipt{}, errors.New("rocketmq local transaction is required")
	}
	message, rmqMessage, err := rocketMQPublishingMessage(ctx, message)
	if err != nil {
		return Receipt{}, err
	}
	if _, ok := p.topics[message.Topic]; !ok {
		return Receipt{}, fmt.Errorf("rocketmq topic %q is not configured", message.Topic)
	}
	transaction := p.producer.BeginTransaction()
	receipts, err := p.producer.SendWithTransaction(ctx, rmqMessage, transaction)
	if err != nil {
		return Receipt{}, fmt.Errorf("send rocketmq half message: %w", err)
	}
	if len(receipts) == 0 {
		rollbackErr := transaction.RollBack()
		return Receipt{}, errors.Join(errors.New("rocketmq transaction returned an empty receipt"), rollbackErr)
	}
	receipt := Receipt{Provider: "rocketmq", ProviderMessageID: receipts[0].MessageID}
	if err := executeLocalTransaction(ctx, local); err != nil {
		rollbackErr := transaction.RollBack()
		return receipt, errors.Join(fmt.Errorf("execute local transaction: %w", err), rollbackErr)
	}
	if err := transaction.Commit(); err != nil {
		return receipt, fmt.Errorf("commit rocketmq transaction message: %w", err)
	}
	return receipt, nil
}

func (p *rocketMQTransactionalPublisher) Close() {
	if p.closed.CompareAndSwap(false, true) {
		_ = p.producer.GracefulStop()
	}
}

func (p *rocketMQTransactionalPublisher) Provider() string { return "rocketmq" }

func executeLocalTransaction(ctx context.Context, local LocalTransaction) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("local transaction panicked")
		}
	}()
	return local(ctx)
}

func decodeRocketMQTransactionCheck(ctx context.Context, view *rmq.MessageView) (Message, error) {
	if view == nil {
		return Message{}, errors.New("nil rocketmq transaction check message")
	}
	source := rocketMQSourceDelivery{
		topic: view.GetTopic(), providerMessageID: view.GetMessageId(), body: append([]byte(nil), view.GetBody()...),
		properties: cloneStringMap(view.GetProperties()), keys: append([]string(nil), view.GetKeys()...),
		deliveryAttempt: view.GetDeliveryAttempt(),
	}
	if value := view.GetTag(); value != nil {
		source.tag = *value
	}
	if value := view.GetMessageGroup(); value != nil {
		source.orderingKey = *value
	}
	return decodeRocketMQDelivery(ctx, source)
}

func toRocketMQResolution(resolution TransactionResolution) rmq.TransactionResolution {
	switch resolution {
	case TransactionCommit:
		return rmq.COMMIT
	case TransactionRollback:
		return rmq.ROLLBACK
	default:
		return rmq.UNKNOWN
	}
}
