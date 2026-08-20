package audit

import (
	"context"
	"strings"
	"testing"
)

func TestWriteRejectsUnserializableDetails(t *testing.T) {
	writer := &Writer{}
	err := writer.Write(context.Background(), Event{Details: map[string]any{"invalid": func() {}}})
	if err == nil || !strings.Contains(err.Error(), "encode audit details") {
		t.Fatalf("Write() error = %v, want audit encoding error", err)
	}
}
