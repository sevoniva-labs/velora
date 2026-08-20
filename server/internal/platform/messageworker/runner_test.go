package messageworker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
)

type fakeTransactioner struct{}

func (fakeTransactioner) WithTx(_ context.Context, fn func(*sql.Tx) error) error { return fn(nil) }

type fakeConsumptionStore struct {
	hashes map[string]string
}

func (f *fakeConsumptionStore) Seen(_ context.Context, _ *sql.Tx, group, eventID, hashValue string) (bool, error) {
	existing, ok := f.hashes[group+"/"+eventID]
	if !ok {
		return false, nil
	}
	if existing != hashValue {
		return false, ErrEventIdentityConflict
	}
	return true, nil
}
func (f *fakeConsumptionStore) Mark(_ context.Context, _ *sql.Tx, group string, delivery messaging.Delivery, hashValue string) error {
	f.hashes[group+"/"+delivery.Message.ID] = hashValue
	return nil
}

type fakeConsumer struct {
	acked    []string
	retried  []string
	receives int
	closed   bool
}

func (f *fakeConsumer) Receive(context.Context) ([]messaging.Delivery, error) {
	f.receives++
	return nil, nil
}
func (f *fakeConsumer) Ack(_ context.Context, delivery messaging.Delivery) error {
	f.acked = append(f.acked, delivery.Message.ID)
	return nil
}
func (f *fakeConsumer) Retry(_ context.Context, delivery messaging.Delivery, _ time.Duration) error {
	f.retried = append(f.retried, delivery.Message.ID)
	return nil
}
func (f *fakeConsumer) Close()           { f.closed = true }
func (f *fakeConsumer) Provider() string { return "rocketmq" }
func (f *fakeConsumer) Group() string    { return "bank-ledger" }

func TestRunnerProcessesOnceAndAcknowledgesDuplicates(t *testing.T) {
	consumer := &fakeConsumer{}
	store := &fakeConsumptionStore{hashes: map[string]string{}}
	handled := 0
	runner, err := newRunner(consumer, fakeTransactioner{}, store, map[string]Handler{
		"account.updated": func(context.Context, *sql.Tx, messaging.Message) error { handled++; return nil },
	}, nil)
	if err != nil {
		t.Fatalf("newRunner() error = %v", err)
	}
	delivery := messaging.Delivery{Message: messaging.Message{ID: "event-1", Topic: "events", Type: "account.updated", Body: []byte(`{"balance":100}`)}}
	if err := runner.process(context.Background(), delivery); err != nil {
		t.Fatalf("first process() error = %v", err)
	}
	if err := runner.process(context.Background(), delivery); err != nil {
		t.Fatalf("duplicate process() error = %v", err)
	}
	if handled != 1 || len(consumer.acked) != 2 || len(consumer.retried) != 0 {
		t.Fatalf("handled=%d acked=%v retried=%v", handled, consumer.acked, consumer.retried)
	}
}

func TestRunnerRetriesIdentityConflictWithoutCallingHandler(t *testing.T) {
	consumer := &fakeConsumer{}
	store := &fakeConsumptionStore{hashes: map[string]string{"bank-ledger/event-1": messageHash(messaging.Message{ID: "event-1", Topic: "events", Type: "account.updated", Body: []byte("old")})}}
	handled := 0
	runner, err := newRunner(consumer, fakeTransactioner{}, store, map[string]Handler{
		"account.updated": func(context.Context, *sql.Tx, messaging.Message) error { handled++; return nil },
	}, nil)
	if err != nil {
		t.Fatalf("newRunner() error = %v", err)
	}
	delivery := messaging.Delivery{Message: messaging.Message{ID: "event-1", Topic: "events", Type: "account.updated", Body: []byte("changed")}, DeliveryAttempt: 3}
	if err := runner.process(context.Background(), delivery); !errors.Is(err, ErrEventIdentityConflict) {
		t.Fatalf("process() error = %v", err)
	}
	if handled != 0 || len(consumer.acked) != 0 || len(consumer.retried) != 1 {
		t.Fatalf("handled=%d acked=%v retried=%v", handled, consumer.acked, consumer.retried)
	}
}

func TestRunnerRetriesHandlerFailure(t *testing.T) {
	consumer := &fakeConsumer{}
	runner, err := newRunner(consumer, fakeTransactioner{}, &fakeConsumptionStore{hashes: map[string]string{}}, map[string]Handler{
		"account.updated": func(context.Context, *sql.Tx, messaging.Message) error { return errors.New("database unavailable") },
	}, nil)
	if err != nil {
		t.Fatalf("newRunner() error = %v", err)
	}
	delivery := messaging.Delivery{Message: messaging.Message{ID: "event-1", Topic: "events", Type: "account.updated"}}
	if err := runner.process(context.Background(), delivery); err == nil {
		t.Fatal("process() accepted handler failure")
	}
	if len(consumer.acked) != 0 || len(consumer.retried) != 1 {
		t.Fatalf("acked=%v retried=%v", consumer.acked, consumer.retried)
	}
}

func TestRunnerBacksOffWhenConsumerReturnsEmptyBatch(t *testing.T) {
	consumer := &fakeConsumer{}
	runner, err := newRunner(consumer, fakeTransactioner{}, &fakeConsumptionStore{hashes: map[string]string{}}, map[string]Handler{
		"account.updated": func(context.Context, *sql.Tx, messaging.Message) error { return nil },
	}, nil)
	if err != nil {
		t.Fatalf("newRunner() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), emptyReceiveDelay/4)
	defer cancel()
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if consumer.receives != 1 || !consumer.closed {
		t.Fatalf("receives=%d closed=%t", consumer.receives, consumer.closed)
	}
}
