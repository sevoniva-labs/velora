package reliablemsg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Event struct {
	ID             string
	OrganizationID string
	Topic          string
	Key            string
	Type           string
	Payload        any
	Headers        map[string]string
	Tag            string
	OrderingKey    string
	DeliverAt      time.Time
}

type Store struct{ db *database.DB }

func New(db *database.DB) *Store { return &Store{db: db} }

const maxBatchErrorDetails = 10

type BatchError struct {
	Failed int
	causes []error
}

func (e *BatchError) Error() string {
	return fmt.Sprintf("%d reliable message(s) failed in the batch: %v", e.Failed, errors.Join(e.causes...))
}

func (e *BatchError) Unwrap() []error { return e.causes }

// EnqueueTx must be called inside the same business transaction as the state
// mutation. This is the local reliable-message atomicity boundary.
func EnqueueTx(ctx context.Context, db *database.DB, tx *sql.Tx, e Event) (string, error) {
	if e.Topic == "" || e.Type == "" {
		return "", errors.New("reliable message: topic and type required")
	}
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return "", err
	}
	headerValues := make(map[string]string, len(e.Headers))
	for rawName, value := range e.Headers {
		name := strings.ToLower(strings.TrimSpace(rawName))
		if _, exists := headerValues[name]; exists {
			return "", fmt.Errorf("reliable message: duplicate header %q", rawName)
		}
		headerValues[name] = value
	}
	if headerValues["traceparent"] == "" {
		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headerValues))
	}
	if err := messaging.Validate(messaging.Message{
		ID: e.ID, OrganizationID: e.OrganizationID, Topic: e.Topic, Key: []byte(e.Key), Type: e.Type,
		Body: payload, Headers: headerValues, Tag: e.Tag, OrderingKey: e.OrderingKey, DeliverAt: e.DeliverAt,
	}); err != nil {
		return "", err
	}
	headers, err := json.Marshal(headerValues)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, db.Rebind(`INSERT INTO reliable_messages(id,organization_id,topic,event_key,event_type,ordering_key,tag,deliver_at,payload_json,headers_json,status,attempts,next_attempt_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), e.ID, nullIfEmpty(e.OrganizationID), e.Topic, e.Key, e.Type, e.OrderingKey, e.Tag, nullTime(e.DeliverAt), string(payload), string(headers), "PENDING", 0, time.Now().UTC(), time.Now().UTC())
	return e.ID, err
}
func (s *Store) Enqueue(ctx context.Context, e Event) (string, error) {
	var id string
	err := s.db.WithTx(ctx, func(tx *sql.Tx) error { var err error; id, err = EnqueueTx(ctx, s.db, tx, e); return err })
	return id, err
}

type pending struct {
	ID, Topic, Key, Type, OrderingKey, Tag, Payload, Headers string
	OrganizationID                                           sql.NullString
	DeliverAt                                                sql.NullTime
	Attempts                                                 int
}

func (s *Store) pending(ctx context.Context, limit int) ([]pending, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, s.db.Rebind(`SELECT id,organization_id,topic,event_key,event_type,ordering_key,tag,deliver_at,payload_json,headers_json,attempts FROM reliable_messages WHERE status='PENDING' AND next_attempt_at<=? ORDER BY created_at LIMIT ?`), time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Topic, &p.Key, &p.Type, &p.OrderingKey, &p.Tag, &p.DeliverAt, &p.Payload, &p.Headers, &p.Attempts); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const processingLease = 5 * time.Minute

func (s *Store) recoverExpiredClaims(ctx context.Context) error {
	// Reuse next_attempt_at as the PROCESSING lease deadline. If a worker dies
	// after claiming an event, another worker can recover it after the lease.
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE reliable_messages SET status='PENDING' WHERE status='PROCESSING' AND next_attempt_at<=?`), time.Now().UTC())
	return err
}

func (s *Store) claim(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE reliable_messages SET status='PROCESSING',next_attempt_at=? WHERE id=? AND status='PENDING'`), time.Now().UTC().Add(processingLease), id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
func (s *Store) published(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE reliable_messages SET status='PUBLISHED',published_at=?,last_error='' WHERE id=?`), time.Now().UTC(), id)
	return err
}
func (s *Store) retry(ctx context.Context, p pending, publishErr error) error {
	attempts := p.Attempts + 1
	status := "PENDING"
	if attempts >= 12 {
		status = "DEAD"
	}
	delay := time.Duration(1<<min(attempts, 8)) * time.Second
	msg := publishErr.Error()
	if len(msg) > 1000 {
		msg = msg[:1000]
	}
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE reliable_messages SET status=?,attempts=?,next_attempt_at=?,last_error=? WHERE id=?`), status, attempts, time.Now().UTC().Add(delay), msg, p.ID)
	return err
}
func (s *Store) PublishBatch(ctx context.Context, bus messaging.Bus, limit int) (int, error) {
	if err := s.recoverExpiredClaims(ctx); err != nil {
		return 0, fmt.Errorf("recover expired reliable message claims: %w", err)
	}
	items, err := s.pending(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("query pending reliable messages: %w", err)
	}
	n := 0
	failed := 0
	failures := make([]error, 0)
	for _, e := range items {
		ok, err := s.claim(ctx, e.ID)
		if err != nil {
			return n, fmt.Errorf("claim reliable message %s: %w", e.ID, err)
		}
		if !ok {
			continue
		}
		message, err := e.message()
		if err != nil {
			failed++
			failures = appendBatchFailure(failures, fmt.Errorf("decode reliable message %s: %w", e.ID, err))
			if stateErr := s.retry(ctx, e, err); stateErr != nil {
				return n, errors.Join(
					newBatchError(failed, failures),
					fmt.Errorf("persist retry state for reliable message %s: %w", e.ID, stateErr),
				)
			}
			continue
		}
		if _, err := bus.Publish(ctx, message); err != nil {
			failed++
			failures = appendBatchFailure(failures, fmt.Errorf("publish reliable message %s: %w", e.ID, err))
			if stateErr := s.retry(ctx, e, err); stateErr != nil {
				return n, errors.Join(
					newBatchError(failed, failures),
					fmt.Errorf("persist retry state for reliable message %s: %w", e.ID, stateErr),
				)
			}
			continue
		}
		if err := s.published(ctx, e.ID); err != nil {
			return n, fmt.Errorf("mark reliable message %s published after provider acceptance; duplicate delivery is possible: %w", e.ID, err)
		}
		n++
	}
	return n, newBatchError(failed, failures)
}

func appendBatchFailure(failures []error, failure error) []error {
	if len(failures) < maxBatchErrorDetails {
		return append(failures, failure)
	}
	return failures
}

func newBatchError(failed int, failures []error) error {
	if failed == 0 {
		return nil
	}
	return &BatchError{Failed: failed, causes: failures}
}

func (p pending) message() (messaging.Message, error) {
	headers := map[string]string{}
	if raw := strings.TrimSpace(p.Headers); raw != "" {
		if err := json.Unmarshal([]byte(raw), &headers); err != nil {
			return messaging.Message{}, fmt.Errorf("decode reliable message headers: %w", err)
		}
	}
	message := messaging.Message{
		ID: p.ID, Topic: p.Topic, Key: []byte(p.Key), Type: p.Type, Body: []byte(p.Payload),
		Headers: headers, Tag: p.Tag, OrderingKey: p.OrderingKey,
	}
	if p.OrganizationID.Valid {
		message.OrganizationID = p.OrganizationID.String
	}
	if p.DeliverAt.Valid {
		message.DeliverAt = p.DeliverAt.Time
	}
	return message, nil
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
