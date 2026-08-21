package crypto

import (
	"context"
	"errors"
	"testing"
)

type fakeKeySource struct {
	activated string
}

func (f *fakeKeySource) Activate(_ context.Context, keyID, version string) error {
	f.activated = keyID + "/" + version
	return nil
}

type fakeDualControl struct {
	called bool
	err    error
}

func (f *fakeDualControl) Authorize(_ context.Context, _, _, _, _, _ string) error {
	f.called = true
	return f.err
}

func TestKeyManagerRequiresTwoPersonApproval(t *testing.T) {
	source := &fakeKeySource{}
	control := &fakeDualControl{}
	manager, err := NewKeyManager(source, control)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Rotate(context.Background(), RotationRequest{ApprovalID: "approval-1", KeyID: "customer-kek", NewVersion: "v2", FirstOperator: "alice", SecondOperator: "alice"}); err == nil {
		t.Fatal("same operator passed dual control")
	}
	if control.called || source.activated != "" {
		t.Fatal("invalid rotation reached an adapter")
	}
	if err := manager.Rotate(context.Background(), RotationRequest{ApprovalID: "approval-1", KeyID: "customer-kek", NewVersion: "v2", FirstOperator: "alice", SecondOperator: "bob"}); err != nil {
		t.Fatal(err)
	}
	if source.activated != "customer-kek/v2" {
		t.Fatalf("activation = %q", source.activated)
	}
}

func TestKeyManagerFailsClosedWhenApprovalAdapterRejects(t *testing.T) {
	source := &fakeKeySource{}
	control := &fakeDualControl{err: errors.New("approval denied")}
	manager, err := NewKeyManager(source, control)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Rotate(context.Background(), RotationRequest{ApprovalID: "approval-1", KeyID: "customer-kek", NewVersion: "v2", FirstOperator: "alice", SecondOperator: "bob"}); err == nil {
		t.Fatal("denied rotation succeeded")
	}
	if source.activated != "" {
		t.Fatal("key was activated after approval denial")
	}
}
