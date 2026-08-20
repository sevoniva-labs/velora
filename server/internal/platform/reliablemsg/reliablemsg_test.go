package reliablemsg

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPendingMessagePreservesReliableEnvelope(t *testing.T) {
	deliverAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	p := pending{
		ID: "event-1", OrganizationID: sql.NullString{String: "org-1", Valid: true}, Topic: "business-events",
		Key: "account-1", Type: "account.updated", OrderingKey: "account-1", Tag: "ACCOUNT",
		Payload: `{"balance":100}`, Headers: `{"correlation-id":"request-1"}`,
		DeliverAt: sql.NullTime{Time: deliverAt, Valid: true},
	}
	message, err := p.message()
	if err != nil {
		t.Fatalf("message() error = %v", err)
	}
	if message.ID != p.ID || message.OrganizationID != "org-1" || message.Type != p.Type {
		t.Fatalf("message identity = %#v", message)
	}
	if message.Headers["correlation-id"] != "request-1" || message.OrderingKey != "account-1" {
		t.Fatalf("message metadata = %#v", message)
	}
	if !message.DeliverAt.Equal(deliverAt) {
		t.Fatalf("DeliverAt = %v, want %v", message.DeliverAt, deliverAt)
	}
}

func TestPendingMessageRejectsMalformedHeaders(t *testing.T) {
	_, err := (pending{Headers: `{`}).message()
	if err == nil {
		t.Fatal("message() accepted malformed persisted headers")
	}
}

func TestBatchErrorRetainsFailureCountAndCauses(t *testing.T) {
	providerErr := errors.New("rocketmq unavailable")
	err := newBatchError(3, []error{providerErr})
	var batchErr *BatchError
	if !errors.As(err, &batchErr) {
		t.Fatalf("newBatchError() type = %T, want *BatchError", err)
	}
	if batchErr.Failed != 3 || !errors.Is(err, providerErr) {
		t.Fatalf("BatchError = %#v, unwrap provider error = %t", batchErr, errors.Is(err, providerErr))
	}
	if !strings.Contains(err.Error(), "3 reliable message(s) failed") {
		t.Fatalf("BatchError message = %q", err.Error())
	}
}

func TestBatchErrorDetailsAreBounded(t *testing.T) {
	failures := make([]error, 0)
	for index := 0; index < maxBatchErrorDetails+5; index++ {
		failures = appendBatchFailure(failures, errors.New("failure"))
	}
	if len(failures) != maxBatchErrorDetails {
		t.Fatalf("failure details = %d, want %d", len(failures), maxBatchErrorDetails)
	}
	if err := newBatchError(0, failures); err != nil {
		t.Fatalf("zero failures returned error %v", err)
	}
}
