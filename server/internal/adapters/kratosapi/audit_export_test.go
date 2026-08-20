package kratosapi

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/app/audit"
)

func TestEncodeAuditCSVSanitizesEveryField(t *testing.T) {
	data, err := encodeAuditCSV([]audit.Event{{
		ID: "=id", OccurredAt: time.Unix(0, 0).UTC(), RequestID: "+request",
		OrganizationID: "-org", ActorID: "@actor", ActorName: "  =name", Action: "+action",
		ResourceType: "-type", ResourceID: "@resource", Result: "=result", ClientIP: "+ip",
		Details: map[string]any{"formula": "=cmd"},
	}})
	if err != nil {
		t.Fatalf("encodeAuditCSV() error = %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatalf("parse encoded CSV: %v", err)
	}
	for index, value := range records[1] {
		trimmed := strings.TrimLeft(value, " \t\r\n")
		if strings.Contains("=+-@", trimmed[:1]) {
			t.Errorf("column %d remains formula-capable: %q", index, value)
		}
	}
}

func TestEncodeAuditExportJSON(t *testing.T) {
	data, err := encodeAuditExport([]audit.Event{{ID: "event-1", Action: "audit.export"}}, "json")
	if err != nil {
		t.Fatalf("encodeAuditExport() error = %v", err)
	}
	if !strings.Contains(string(data), `"action":"audit.export"`) {
		t.Fatalf("JSON export missing event: %s", data)
	}
}
