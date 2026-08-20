package configchange

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type testAuditor struct{ err error }

func (a testAuditor) Record(context.Context, AuditEntry) error { return a.err }

func testChange(t *testing.T) Change {
	t.Helper()
	change, err := New("change-1", "org-1", "prod", "DEFAULT_GROUP", "forge.yaml", strings.Repeat("a", 64), "vault://config/change-1", "creator-1", 2, 1, true, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return change
}

func TestConfigChangeRequiresIndependentApprovalAndSupportsRollback(t *testing.T) {
	change := testChange(t)
	at := time.Date(2026, 8, 20, 10, 1, 0, 0, time.UTC)
	if _, _, err := Apply(change, Request{Action: ActionApprove, At: at, ActorID: "creator-1", ApprovalID: "approval-1"}); err == nil {
		t.Fatal("creator must not approve own config change")
	}
	var err error
	change, _, err = Apply(change, Request{Action: ActionApprove, At: at, ActorID: "approver-1", ApprovalID: "approval-1"})
	if err != nil {
		t.Fatal(err)
	}
	change, _, err = Apply(change, Request{Action: ActionPublish, At: at.Add(time.Minute), ActorID: "approver-1", ApprovalID: "approval-1"})
	if err != nil || change.State != StatePublished {
		t.Fatalf("publish = %#v, err=%v", change, err)
	}
	change, _, err = Apply(change, Request{Action: ActionRequestRollback, At: at.Add(2 * time.Minute), ActorID: "operator-2", ApprovalID: "approval-rollback"})
	if err != nil || change.State != StateRollbackPending {
		t.Fatalf("rollback request = %#v, err=%v", change, err)
	}
	change, _, err = Apply(change, Request{Action: ActionRollback, At: at.Add(3 * time.Minute), ActorID: "operator-2", ApprovalID: "approval-rollback"})
	if err != nil || change.State != StateRolledBack {
		t.Fatalf("rollback = %#v, err=%v", change, err)
	}
}

func TestConfigChangeAuditFailureFailsClosed(t *testing.T) {
	change := testChange(t)
	_, err := ApplyWithAudit(context.Background(), testAuditor{err: errors.New("audit unavailable")}, change, Request{Action: ActionApprove, At: time.Now().UTC(), ActorID: "approver-1", ApprovalID: "approval-1"})
	if err == nil {
		t.Fatal("audit failure must fail closed")
	}
}
