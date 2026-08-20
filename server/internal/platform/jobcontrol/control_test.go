package jobcontrol

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingAuditor struct {
	entry AuditEntry
	err   error
}

func (a *recordingAuditor) Record(_ context.Context, entry AuditEntry) error {
	a.entry = entry
	return a.err
}

func testDefinition() Definition {
	return Definition{Name: "settlement-close", MaxAttempts: 3, RetryBaseDelay: time.Second, RetryMaxDelay: time.Minute, RequiresApproval: true, RequiresCompensation: true}
}

func testRun(def Definition) Run {
	return Run{ID: "run-1", JobName: def.Name, OrganizationID: "org-1", IdempotencyKey: "settlement-2026-08-20", State: StatePending, MaxAttempts: def.MaxAttempts}
}

func transition(t *testing.T, def Definition, run Run, action Action, approval string) Run {
	t.Helper()
	next, _, err := Apply(def, run, Request{Action: action, At: time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC), ActorID: "operator-1", ApprovalID: approval, Cause: "provider unavailable"})
	if err != nil {
		t.Fatalf("Apply(%s): %v", action, err)
	}
	return next
}

func TestApplyEnforcesApprovalRetryAndDeadLetter(t *testing.T) {
	def := testDefinition()
	run := testRun(def)
	if _, _, err := Apply(def, run, Request{Action: ActionStart, At: time.Now().UTC(), ActorID: "operator-1"}); err == nil {
		t.Fatal("approved job start must reject missing approval")
	}
	run = transition(t, def, run, ActionStart, "approval-start")
	for i := 0; i < 2; i++ {
		failed, _, err := Apply(def, run, Request{Action: ActionFail, At: time.Date(2026, 8, 20, 10, i+1, 0, 0, time.UTC), ActorID: "worker-1", Cause: "provider unavailable"})
		if err != nil {
			t.Fatalf("failure transition: %v", err)
		}
		run = failed
		run, _, err = Apply(def, run, Request{Action: ActionRetry, At: time.Date(2026, 8, 20, 11+i, 0, 0, 0, time.UTC), ActorID: "worker-1"})
		if err != nil {
			t.Fatalf("retry transition: %v", err)
		}
	}
	dead, _, err := Apply(def, run, Request{Action: ActionFail, At: time.Date(2026, 8, 20, 10, 3, 0, 0, time.UTC), ActorID: "worker-1", Cause: "provider unavailable"})
	if err != nil || dead.State != StateDeadLetter {
		t.Fatalf("final failure = %#v, err=%v; want dead letter", dead, err)
	}
	compensating := transition(t, def, dead, ActionRequestCompensation, "approval-compensation")
	if compensating.State != StateCompensationPending {
		t.Fatalf("compensation state = %q", compensating.State)
	}
}

func TestApplyWithAuditFailsClosed(t *testing.T) {
	def := testDefinition()
	run := testRun(def)
	if _, err := ApplyWithAudit(context.Background(), nil, def, run, Request{}); err == nil {
		t.Fatal("missing auditor must fail closed")
	}
	auditor := &recordingAuditor{err: errors.New("audit store unavailable")}
	run = transition(t, def, run, ActionStart, "approval-start")
	_, err := ApplyWithAudit(context.Background(), auditor, def, run, Request{Action: ActionSucceed, At: time.Now().UTC(), ActorID: "worker-1"})
	if err == nil || auditor.entry.After != StateSucceeded {
		t.Fatalf("audit failure = %v, entry = %#v", err, auditor.entry)
	}
}

func TestRetryBackoffIsBounded(t *testing.T) {
	def := Definition{Name: "job", MaxAttempts: 100, RetryBaseDelay: time.Second, RetryMaxDelay: 3 * time.Second}
	if got := retryDelay(def, 10); got != 3*time.Second {
		t.Fatalf("retry delay = %v; want 3s", got)
	}
}
