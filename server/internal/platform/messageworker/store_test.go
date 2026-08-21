package messageworker

import (
	"testing"

	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
)

func TestMessageHashSeparatesIdentityBoundaries(t *testing.T) {
	base := messaging.Message{ID: "event-1", OrganizationID: "org-1", Topic: "events", Type: "account.updated", Key: []byte("account-1"), Body: []byte("payload")}
	baseHash := messageHash(base)
	if baseHash == "" || baseHash != messageHash(base) {
		t.Fatal("messageHash() is not deterministic")
	}
	changed := base
	changed.Body = []byte("different")
	if messageHash(base) == messageHash(changed) {
		t.Fatal("messageHash() ignored body changes")
	}
	changed = base
	changed.Type = "account.closed"
	if messageHash(base) == messageHash(changed) {
		t.Fatal("messageHash() ignored event type changes")
	}
}
