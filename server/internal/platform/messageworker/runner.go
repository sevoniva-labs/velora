package messageworker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Handler func(context.Context, *sql.Tx, messaging.Message) error

const emptyReceiveDelay = 100 * time.Millisecond

type Runner struct {
	consumer messaging.Consumer
	tx       transactioner
	store    consumptionStore
	handlers map[string]Handler
	logger   *slog.Logger
}

func New(db *database.DB, consumer messaging.Consumer, handlers map[string]Handler, logger *slog.Logger) (*Runner, error) {
	if db == nil {
		return nil, errors.New("message worker database is required")
	}
	return newRunner(consumer, db, &sqlConsumptionStore{db: db}, handlers, logger)
}

func newRunner(consumer messaging.Consumer, tx transactioner, store consumptionStore, handlers map[string]Handler, logger *slog.Logger) (*Runner, error) {
	if consumer == nil || tx == nil || store == nil {
		return nil, errors.New("message worker dependencies are required")
	}
	if strings.TrimSpace(consumer.Group()) == "" {
		return nil, errors.New("message worker consumer group is required")
	}
	registered := make(map[string]Handler, len(handlers))
	for rawType, handler := range handlers {
		eventType := strings.TrimSpace(rawType)
		if eventType == "" || handler == nil {
			return nil, errors.New("message worker handler type and function are required")
		}
		registered[eventType] = handler
	}
	if len(registered) == 0 {
		return nil, errors.New("message worker requires at least one handler")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{consumer: consumer, tx: tx, store: store, handlers: registered, logger: logger}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	defer r.consumer.Close()
	receiveFailures := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		deliveries, err := r.consumer.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			receiveFailures++
			delay := retryDelay(int32(receiveFailures))
			r.logger.Error("message receive failed", "provider", r.consumer.Provider(), "group", r.consumer.Group(), "retry_in", delay, "err", err)
			if err := waitContext(ctx, delay); err != nil {
				return nil
			}
			continue
		}
		receiveFailures = 0
		if len(deliveries) == 0 {
			if err := waitContext(ctx, emptyReceiveDelay); err != nil {
				return nil
			}
			continue
		}
		for _, delivery := range deliveries {
			if err := r.process(ctx, delivery); err != nil && ctx.Err() == nil {
				r.logger.Error("message processing failed", "provider", r.consumer.Provider(), "group", r.consumer.Group(),
					"event_id", delivery.Message.ID, "provider_message_id", delivery.ProviderMessageID,
					"attempt", delivery.DeliveryAttempt, "err", err)
			}
		}
	}
}

func (r *Runner) process(ctx context.Context, delivery messaging.Delivery) error {
	if delivery.DecodeError != nil {
		return r.retry(ctx, delivery, fmt.Errorf("malformed message: %w", delivery.DecodeError))
	}
	handler, ok := r.handlers[delivery.Message.Type]
	if !ok {
		return r.retry(ctx, delivery, fmt.Errorf("no handler registered for event type %q", delivery.Message.Type))
	}

	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(delivery.Message.Headers))
	ctx, span := otel.Tracer("github.com/sevoniva-labs/velora/server/messageworker").Start(ctx, "messaging.consume",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", r.consumer.Provider()),
			attribute.String("messaging.destination.name", delivery.Message.Topic),
			attribute.String("messaging.operation.type", "process"),
			attribute.String("messaging.message.id", delivery.Message.ID),
			attribute.String("messaging.consumer.group.name", r.consumer.Group()),
		),
	)
	defer span.End()

	hashValue := messageHash(delivery.Message)
	duplicate := false
	err := r.tx.WithTx(ctx, func(tx *sql.Tx) error {
		seen, err := r.store.Seen(ctx, tx, r.consumer.Group(), delivery.Message.ID, hashValue)
		if err != nil {
			return err
		}
		if seen {
			duplicate = true
			return nil
		}
		if err := handler(ctx, tx, delivery.Message); err != nil {
			return fmt.Errorf("handle event %q: %w", delivery.Message.Type, err)
		}
		return r.store.Mark(ctx, tx, r.consumer.Group(), delivery, hashValue)
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "transactional message handling failed")
		return r.retry(ctx, delivery, err)
	}
	span.SetAttributes(attribute.Bool("messaging.message.duplicate", duplicate))
	if err := r.consumer.Ack(ctx, delivery); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "message acknowledgement failed")
		return err
	}
	return nil
}

func (r *Runner) retry(ctx context.Context, delivery messaging.Delivery, cause error) error {
	delay := retryDelay(delivery.DeliveryAttempt)
	if err := r.consumer.Retry(ctx, delivery, delay); err != nil {
		return errors.Join(cause, fmt.Errorf("schedule message retry: %w", err))
	}
	return cause
}

func retryDelay(attempt int32) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		attempt = 9
	}
	return time.Duration(1<<uint(attempt-1)) * time.Second
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
