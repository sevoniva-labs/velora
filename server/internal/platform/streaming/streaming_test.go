package streaming

import (
	"context"
	"strings"
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

func TestPrepareRecordNormalizesHeadersWithoutBusinessEnvelope(t *testing.T) {
	record, headers, err := prepareRecord(context.Background(), Record{
		Stream: " ledger-postings ", Key: []byte("account-1"), Value: []byte("entry"),
		Headers: map[string]string{"Correlation-ID": "request-1"},
	})
	if err != nil {
		t.Fatalf("prepareRecord() error = %v", err)
	}
	if record.Stream != "ledger-postings" || headers["correlation-id"] != "request-1" {
		t.Fatalf("record = %#v, headers = %#v", record, headers)
	}
	if _, exists := headers["x-forge-event-id"]; exists {
		t.Fatal("stream record unexpectedly received a business-message identity")
	}
}

func TestPrepareRecordRejectsAmbiguousHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
	}{
		{name: "space", headers: map[string]string{"bad header": "value"}},
		{name: "colon", headers: map[string]string{"bad:header": "value"}},
		{name: "non ascii", headers: map[string]string{"追踪": "value"}},
		{name: "too long", headers: map[string]string{strings.Repeat("a", maxHeaderNameBytes+1): "value"}},
		{name: "normalization collision", headers: map[string]string{"Correlation-ID": "one", "correlation-id": "two"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := prepareRecord(context.Background(), Record{Stream: "audit-events", Headers: test.headers}); err == nil {
				t.Fatal("prepareRecord() accepted ambiguous headers")
			}
		})
	}
}

func TestDisabledStreamingDoesNotSilentlyDrop(t *testing.T) {
	producer, err := New(config.Streaming{Provider: "disabled"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := producer.Append(context.Background(), Record{Stream: "events"}); err == nil {
		t.Fatal("disabled streaming provider accepted a record")
	}
}
