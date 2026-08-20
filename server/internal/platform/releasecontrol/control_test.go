package releasecontrol

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReleaseLifecycleRequiresDualControlAndEvidence(t *testing.T) {
	store := &fakeStore{records: map[string]ReleaseRecord{}}
	controller, err := NewController(store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	record, err := controller.Create(context.Background(), ReleaseRequest{
		ID: "rel-1", Version: "2026.08.20.1", TargetEnvironment: "production",
		ArtifactDigest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		SBOMDigest:     "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		SourceCommit:   "abcdef0123456", ProvenanceEvidence: "attestation-1", TestEvidence: "ci-1",
		VulnerabilityStatus: "passed", LicenseStatus: "passed", RollbackPlan: "rollback-1",
		RequestedBy: "maker", RequiredApprovals: 2, WindowStart: now.Add(-time.Minute), WindowEnd: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Approve(context.Background(), record.ID, "maker"); err == nil {
		t.Fatal("requester approved its own release")
	}
	if _, err := controller.RequestApproval(context.Background(), record.ID, "maker"); err != nil {
		t.Fatal(err)
	}
	if record, err = controller.Approve(context.Background(), record.ID, "approver-a"); err != nil || record.State != StatePendingApproval {
		t.Fatalf("first approval did not remain pending: %#v %v", record, err)
	}
	if record, err = controller.Approve(context.Background(), record.ID, "approver-b"); err != nil || record.State != StateApproved {
		t.Fatalf("second approval did not approve: %#v %v", record, err)
	}
	if record, err = controller.Start(context.Background(), record.ID, "operator"); err != nil || record.State != StateExecuting {
		t.Fatalf("release did not start: %#v %v", record, err)
	}
	if record, err = controller.Succeed(context.Background(), record.ID, "operator", "deploy-1"); err != nil || record.State != StateSucceeded {
		t.Fatalf("release did not succeed: %#v %v", record, err)
	}
	if record, err = controller.RequestRollback(context.Background(), record.ID, "operator", "failed business verification"); err != nil || record.State != StateRollbackRequested {
		t.Fatalf("rollback was not requested: %#v %v", record, err)
	}
	if record, err = controller.ApproveRollback(context.Background(), record.ID, "approver-a"); err != nil || record.State != StateRollbackRequested {
		t.Fatalf("first rollback approval did not remain pending: %#v %v", record, err)
	}
	if record, err = controller.ApproveRollback(context.Background(), record.ID, "approver-b"); err != nil || record.State != StateRollbackApproved {
		t.Fatalf("second rollback approval did not approve: %#v %v", record, err)
	}
	if record, err = controller.StartRollback(context.Background(), record.ID, "operator"); err != nil || record.State != StateRollingBack {
		t.Fatalf("rollback did not start: %#v %v", record, err)
	}
	if record, err = controller.CompleteRollback(context.Background(), record.ID, "operator", "rollback-verified"); err != nil || record.State != StateRolledBack {
		t.Fatalf("rollback did not complete: %#v %v", record, err)
	}
	if len(store.events) != 11 {
		t.Fatalf("expected one audit event per transition, got %d", len(store.events))
	}
}

func TestReleaseRejectsMissingEvidenceAndWindow(t *testing.T) {
	controller, err := NewController(&fakeStore{records: map[string]ReleaseRecord{}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.Create(context.Background(), ReleaseRequest{ID: "rel-2", Version: "v1", RequestedBy: "maker", RequiredApprovals: 1})
	if err == nil {
		t.Fatal("accepted incomplete release evidence")
	}
}

type fakeStore struct {
	records map[string]ReleaseRecord
	events  []AuditEvent
}

func (f *fakeStore) Create(_ context.Context, record ReleaseRecord, event AuditEvent) error {
	if _, exists := f.records[record.ID]; exists {
		return errors.New("duplicate release")
	}
	f.records[record.ID] = record
	f.events = append(f.events, event)
	return nil
}
func (f *fakeStore) Get(_ context.Context, id string) (ReleaseRecord, error) {
	record, ok := f.records[id]
	if !ok {
		return ReleaseRecord{}, errors.New("not found")
	}
	return record, nil
}
func (f *fakeStore) Transition(_ context.Context, id string, expected State, next ReleaseRecord, event AuditEvent) error {
	record, ok := f.records[id]
	if !ok || record.State != expected {
		return errors.New("state conflict")
	}
	f.records[id] = next
	f.events = append(f.events, event)
	return nil
}
