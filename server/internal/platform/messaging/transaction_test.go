package messaging

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

type fakeRocketMQTransaction struct {
	commits   int
	rollbacks int
}

func (f *fakeRocketMQTransaction) Commit() error   { f.commits++; return nil }
func (f *fakeRocketMQTransaction) RollBack() error { f.rollbacks++; return nil }

type fakeRocketMQTransactionProducer struct {
	transaction *fakeRocketMQTransaction
	sendErr     error
	sends       int
	started     int
	stopped     int
}

func (f *fakeRocketMQTransactionProducer) Start() error                      { f.started++; return nil }
func (f *fakeRocketMQTransactionProducer) GracefulStop() error               { f.stopped++; return nil }
func (f *fakeRocketMQTransactionProducer) BeginTransaction() rmq.Transaction { return f.transaction }
func (f *fakeRocketMQTransactionProducer) SendWithTransaction(context.Context, *rmq.Message, rmq.Transaction) ([]*rmq.SendReceipt, error) {
	f.sends++
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return []*rmq.SendReceipt{{MessageID: "rmq-transaction-1"}}, nil
}

func TestTransactionalPublisherCommitsAfterLocalSuccess(t *testing.T) {
	transaction := &fakeRocketMQTransaction{}
	producer := &fakeRocketMQTransactionProducer{transaction: transaction}
	publisher, err := newRocketMQTransactionalPublisherWithFactory(transactionConfig(), func(context.Context, Message) (TransactionResolution, error) {
		return TransactionCommit, nil
	}, func(config.Messaging, *tls.Config, []string, *rmq.TransactionChecker) (rocketMQTransactionProducer, error) {
		return producer, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQTransactionalPublisherWithFactory() error = %v", err)
	}
	localCalls := 0
	receipt, err := publisher.PublishTransaction(context.Background(), transactionMessage(), func(context.Context) error {
		localCalls++
		return nil
	})
	if err != nil {
		t.Fatalf("PublishTransaction() error = %v", err)
	}
	if receipt.ProviderMessageID == "" || localCalls != 1 || transaction.commits != 1 || transaction.rollbacks != 0 {
		t.Fatalf("receipt=%#v local=%d commits=%d rollbacks=%d", receipt, localCalls, transaction.commits, transaction.rollbacks)
	}
	publisher.Close()
}

func TestTransactionalPublisherRollsBackAfterLocalFailure(t *testing.T) {
	transaction := &fakeRocketMQTransaction{}
	producer := &fakeRocketMQTransactionProducer{transaction: transaction}
	publisher, err := newRocketMQTransactionalPublisherWithFactory(transactionConfig(), func(context.Context, Message) (TransactionResolution, error) {
		return TransactionRollback, nil
	}, func(config.Messaging, *tls.Config, []string, *rmq.TransactionChecker) (rocketMQTransactionProducer, error) {
		return producer, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQTransactionalPublisherWithFactory() error = %v", err)
	}
	defer publisher.Close()
	_, err = publisher.PublishTransaction(context.Background(), transactionMessage(), func(context.Context) error {
		return errors.New("business rollback")
	})
	if err == nil || transaction.rollbacks != 1 || transaction.commits != 0 {
		t.Fatalf("error=%v commits=%d rollbacks=%d", err, transaction.commits, transaction.rollbacks)
	}
}

func TestTransactionalPublisherDoesNotRunLocalWorkWhenHalfSendFails(t *testing.T) {
	transaction := &fakeRocketMQTransaction{}
	producer := &fakeRocketMQTransactionProducer{transaction: transaction, sendErr: errors.New("proxy unavailable")}
	publisher, err := newRocketMQTransactionalPublisherWithFactory(transactionConfig(), func(context.Context, Message) (TransactionResolution, error) {
		return TransactionUnknown, nil
	}, func(config.Messaging, *tls.Config, []string, *rmq.TransactionChecker) (rocketMQTransactionProducer, error) {
		return producer, nil
	})
	if err != nil {
		t.Fatalf("newRocketMQTransactionalPublisherWithFactory() error = %v", err)
	}
	defer publisher.Close()
	localCalls := 0
	_, err = publisher.PublishTransaction(context.Background(), transactionMessage(), func(context.Context) error {
		localCalls++
		return nil
	})
	if err == nil || localCalls != 0 {
		t.Fatalf("error=%v local calls=%d", err, localCalls)
	}
}

func transactionConfig() config.Messaging {
	return config.Messaging{Provider: "rocketmq", RocketMQTopics: []string{"financial-events"}}
}

func transactionMessage() Message {
	return Message{ID: "event-1", Topic: "financial-events", Type: "ledger.posted", Body: []byte(`{"amount":100}`)}
}
