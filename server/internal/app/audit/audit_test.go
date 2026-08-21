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

func TestAuditIntegrityQuerySkipsArchivedPrefix(t *testing.T) {
	query, args := auditIntegrityQuery("org-1", 42, true)
	if !strings.Contains(query, "AND sequence_no>?") {
		t.Fatalf("anchored integrity query does not skip archived prefix: %s", query)
	}
	if len(args) != 3 || args[2] != int64(42) {
		t.Fatalf("anchored query args = %#v", args)
	}
	query, args = auditIntegrityQuery("org-1", 42, false)
	if strings.Contains(query, "sequence_no>?") || len(args) != 2 {
		t.Fatalf("unanchored integrity query unexpectedly skips rows: %s %#v", query, args)
	}
}
