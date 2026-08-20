package releasecontrol

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type State string

const (
	StateDraft             State = "Draft"
	StatePendingApproval   State = "PendingApproval"
	StateApproved          State = "Approved"
	StateExecuting         State = "Executing"
	StateSucceeded         State = "Succeeded"
	StateFailed            State = "Failed"
	StateRollbackRequested State = "RollbackRequested"
	StateRollbackApproved  State = "RollbackApproved"
	StateRollingBack       State = "RollingBack"
	StateRolledBack        State = "RolledBack"
)

type ReleaseRequest struct {
	ID                  string
	Version             string
	TargetEnvironment   string
	ArtifactDigest      string
	SBOMDigest          string
	SourceCommit        string
	ProvenanceEvidence  string
	TestEvidence        string
	VulnerabilityStatus string
	LicenseStatus       string
	RollbackPlan        string
	RequestedBy         string
	RequiredApprovals   int
	WindowStart         time.Time
	WindowEnd           time.Time
}

type ReleaseRecord struct {
	ReleaseRequest
	State              State
	Approvers          []string
	RollbackApprovers  []string
	DeploymentEvidence string
	RollbackEvidence   string
	FailureReason      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AuditEvent struct {
	ReleaseID string
	Event     string
	Actor     string
	From      State
	To        State
	At        time.Time
}

// ReleaseStore must persist a state transition and its audit event in one
// local database transaction. This is deliberately not an in-memory workflow
// or a best-effort log call.
type ReleaseStore interface {
	Create(context.Context, ReleaseRecord, AuditEvent) error
	Get(context.Context, string) (ReleaseRecord, error)
	Transition(context.Context, string, State, ReleaseRecord, AuditEvent) error
}

type Controller struct {
	store ReleaseStore
	now   func() time.Time
}

func NewController(store ReleaseStore) (*Controller, error) {
	if store == nil {
		return nil, errors.New("release store is required")
	}
	return &Controller{store: store, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (c *Controller) Create(ctx context.Context, request ReleaseRequest) (ReleaseRecord, error) {
	if err := validateReleaseRequest(request); err != nil {
		return ReleaseRecord{}, err
	}
	now := c.now().UTC()
	record := ReleaseRecord{ReleaseRequest: request, State: StateDraft, CreatedAt: now, UpdatedAt: now}
	event := AuditEvent{ReleaseID: record.ID, Event: "release.created", Actor: request.RequestedBy, To: StateDraft, At: now}
	if err := c.store.Create(ctx, record, event); err != nil {
		return ReleaseRecord{}, fmt.Errorf("create release: %w", err)
	}
	return record, nil
}

func (c *Controller) RequestApproval(ctx context.Context, id, actor string) (ReleaseRecord, error) {
	return c.transition(ctx, id, StateDraft, actor, "release.approval_requested", func(record ReleaseRecord) ReleaseRecord { return record }, StatePendingApproval)
}

func (c *Controller) Approve(ctx context.Context, id, approver string) (ReleaseRecord, error) {
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StatePendingApproval {
		return ReleaseRecord{}, fmt.Errorf("release is not awaiting approval: %s", record.State)
	}
	if strings.TrimSpace(approver) == "" || approver == record.RequestedBy {
		return ReleaseRecord{}, errors.New("requester cannot approve its own release")
	}
	if contains(record.Approvers, approver) {
		return ReleaseRecord{}, errors.New("approver has already approved this release")
	}
	next := record
	next.Approvers = append(append([]string(nil), record.Approvers...), approver)
	to := StatePendingApproval
	if len(next.Approvers) >= record.RequiredApprovals {
		to = StateApproved
	}
	next.State = to
	next.UpdatedAt = c.now().UTC()
	event := AuditEvent{ReleaseID: id, Event: "release.approved", Actor: approver, From: record.State, To: to, At: next.UpdatedAt}
	if err := c.store.Transition(ctx, id, record.State, next, event); err != nil {
		return ReleaseRecord{}, fmt.Errorf("approve release: %w", err)
	}
	return next, nil
}

func (c *Controller) Start(ctx context.Context, id, actor string) (ReleaseRecord, error) {
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StateApproved {
		return ReleaseRecord{}, fmt.Errorf("release is not approved: %s", record.State)
	}
	now := c.now().UTC()
	if now.Before(record.WindowStart) || !now.Before(record.WindowEnd) {
		return ReleaseRecord{}, errors.New("release is outside the approved change window")
	}
	return c.transitionAt(ctx, record, actor, "release.started", StateExecuting, now, func(next ReleaseRecord) ReleaseRecord { return next })
}

func (c *Controller) Succeed(ctx context.Context, id, actor, evidence string) (ReleaseRecord, error) {
	return c.finish(ctx, id, actor, evidence, "release.succeeded", StateSucceeded)
}

func (c *Controller) Fail(ctx context.Context, id, actor, reason string) (ReleaseRecord, error) {
	if strings.TrimSpace(reason) == "" {
		return ReleaseRecord{}, errors.New("release failure reason is required")
	}
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StateExecuting {
		return ReleaseRecord{}, fmt.Errorf("release is not executing: %s", record.State)
	}
	return c.transitionAt(ctx, record, actor, "release.failed", StateFailed, c.now().UTC(), func(next ReleaseRecord) ReleaseRecord {
		next.FailureReason = reason
		return next
	})
}

func (c *Controller) RequestRollback(ctx context.Context, id, actor, reason string) (ReleaseRecord, error) {
	if strings.TrimSpace(reason) == "" {
		return ReleaseRecord{}, errors.New("rollback reason is required")
	}
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StateSucceeded {
		return ReleaseRecord{}, fmt.Errorf("release is not eligible for rollback: %s", record.State)
	}
	return c.transitionAt(ctx, record, actor, "release.rollback_requested", StateRollbackRequested, c.now().UTC(), func(next ReleaseRecord) ReleaseRecord {
		next.FailureReason = reason
		next.RollbackApprovers = nil
		return next
	})
}

func (c *Controller) ApproveRollback(ctx context.Context, id, approver string) (ReleaseRecord, error) {
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StateRollbackRequested {
		return ReleaseRecord{}, fmt.Errorf("rollback is not awaiting approval: %s", record.State)
	}
	if strings.TrimSpace(approver) == "" || approver == record.RequestedBy {
		return ReleaseRecord{}, errors.New("requester cannot approve its own rollback")
	}
	if contains(record.RollbackApprovers, approver) {
		return ReleaseRecord{}, errors.New("rollback approver has already approved this release")
	}
	next := record
	next.RollbackApprovers = append(append([]string(nil), record.RollbackApprovers...), approver)
	to := StateRollbackRequested
	if len(next.RollbackApprovers) >= record.RequiredApprovals {
		to = StateRollbackApproved
	}
	next.State = to
	next.UpdatedAt = c.now().UTC()
	event := AuditEvent{ReleaseID: id, Event: "release.rollback_approved", Actor: approver, From: record.State, To: to, At: next.UpdatedAt}
	if err := c.store.Transition(ctx, id, record.State, next, event); err != nil {
		return ReleaseRecord{}, fmt.Errorf("approve rollback: %w", err)
	}
	return next, nil
}

func (c *Controller) StartRollback(ctx context.Context, id, actor string) (ReleaseRecord, error) {
	return c.transition(ctx, id, StateRollbackApproved, actor, "release.rollback_started", func(record ReleaseRecord) ReleaseRecord { return record }, StateRollingBack)
}

func (c *Controller) CompleteRollback(ctx context.Context, id, actor, evidence string) (ReleaseRecord, error) {
	if strings.TrimSpace(evidence) == "" {
		return ReleaseRecord{}, errors.New("rollback evidence is required")
	}
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StateRollingBack {
		return ReleaseRecord{}, fmt.Errorf("rollback is not executing: %s", record.State)
	}
	return c.transitionAt(ctx, record, actor, "release.rollback_completed", StateRolledBack, c.now().UTC(), func(next ReleaseRecord) ReleaseRecord {
		next.RollbackEvidence = evidence
		return next
	})
}

func (c *Controller) finish(ctx context.Context, id, actor, evidence, event string, state State) (ReleaseRecord, error) {
	if strings.TrimSpace(evidence) == "" {
		return ReleaseRecord{}, errors.New("deployment evidence is required")
	}
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != StateExecuting {
		return ReleaseRecord{}, fmt.Errorf("release is not executing: %s", record.State)
	}
	return c.transitionAt(ctx, record, actor, event, state, c.now().UTC(), func(next ReleaseRecord) ReleaseRecord {
		next.DeploymentEvidence = evidence
		return next
	})
}

func (c *Controller) transition(ctx context.Context, id string, expected State, actor, event string, mutate func(ReleaseRecord) ReleaseRecord, state State) (ReleaseRecord, error) {
	record, err := c.store.Get(ctx, id)
	if err != nil {
		return ReleaseRecord{}, err
	}
	if record.State != expected {
		return ReleaseRecord{}, fmt.Errorf("release state is %s, expected %s", record.State, expected)
	}
	return c.transitionAt(ctx, record, actor, event, state, c.now().UTC(), mutate)
}

func (c *Controller) transitionAt(ctx context.Context, record ReleaseRecord, actor, event string, state State, at time.Time, mutate func(ReleaseRecord) ReleaseRecord) (ReleaseRecord, error) {
	if strings.TrimSpace(actor) == "" {
		return ReleaseRecord{}, errors.New("release actor is required")
	}
	next := mutate(record)
	next.State = state
	next.UpdatedAt = at
	audit := AuditEvent{ReleaseID: record.ID, Event: event, Actor: actor, From: record.State, To: state, At: at}
	if err := c.store.Transition(ctx, record.ID, record.State, next, audit); err != nil {
		return ReleaseRecord{}, fmt.Errorf("release state transition %s -> %s: %w", record.State, state, err)
	}
	return next, nil
}

func validateReleaseRequest(request ReleaseRequest) error {
	fields := map[string]string{
		"release id": request.ID, "version": request.Version, "target environment": request.TargetEnvironment,
		"source commit": request.SourceCommit, "provenance evidence": request.ProvenanceEvidence,
		"test evidence": request.TestEvidence, "rollback plan": request.RollbackPlan, "requested by": request.RequestedBy,
	}
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if !validDigest(request.ArtifactDigest) || !validDigest(request.SBOMDigest) {
		return errors.New("artifact and sbom digests must be sha256 hex digests")
	}
	if request.VulnerabilityStatus != "passed" || request.LicenseStatus != "passed" {
		return errors.New("vulnerability and license evidence must be passed")
	}
	if request.RequiredApprovals < 2 {
		return errors.New("production releases require at least two approvals")
	}
	if request.WindowStart.IsZero() || request.WindowEnd.IsZero() || !request.WindowEnd.After(request.WindowStart) {
		return errors.New("a valid release window is required")
	}
	return nil
}

func validDigest(value string) bool {
	value = strings.TrimPrefix(strings.TrimSpace(value), "sha256:")
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func contains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
