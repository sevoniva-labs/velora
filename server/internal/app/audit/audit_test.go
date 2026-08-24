package audit

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWriteRejectsUnserializableDetails(t *testing.T) {
	writer := &Writer{}
	err := writer.Write(context.Background(), Event{Details: map[string]any{"invalid": func() {}}})
	if err == nil || !strings.Contains(err.Error(), "encode audit details") {
		t.Fatalf("Write() error = %v, want audit encoding error", err)
	}
}

func TestAuditListWhereIncludesEveryFilter(t *testing.T) {
	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	to := from.Add(24 * time.Hour)
	query, args := auditListWhere("org-1", ListQuery{Operator: " Carson ", Action: "auth.login", ResourceType: "user", ResourceID: "user-1", Result: "success", From: &from, To: &to})
	for _, fragment := range []string{"organization_id=?", "LOWER(actor_name) LIKE ?", "action=?", "resource_type=?", "resource_id=?", "result=?", "occurred_at>=?", "occurred_at<=?"} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("audit query %q missing %q", query, fragment)
		}
	}
	if len(args) != 9 || args[0] != "org-1" || args[1] != "%carson%" || args[3] != "auth.login" || args[4] != "user" || args[5] != "user-1" || args[6] != "SUCCESS" {
		t.Fatalf("audit query args = %#v", args)
	}
	if args[7] != from.UTC() || args[8] != to.UTC() {
		t.Fatalf("audit time bounds are not UTC: %#v", args)
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
