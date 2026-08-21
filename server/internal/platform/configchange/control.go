package configchange

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type State string

const (
	StatePendingApproval State = "PENDING_APPROVAL"
	StateApproved        State = "APPROVED"
	StatePublished       State = "PUBLISHED"
	StateRollbackPending State = "ROLLBACK_PENDING"
	StateRolledBack      State = "ROLLED_BACK"
	StateRejected        State = "REJECTED"
)

type Action string

const (
	ActionApprove         Action = "APPROVE"
	ActionPublish         Action = "PUBLISH"
	ActionRequestRollback Action = "REQUEST_ROLLBACK"
	ActionRollback        Action = "ROLLBACK"
	ActionReject          Action = "REJECT"
)

type Change struct {
	ID                      string
	OrganizationID          string
	Namespace               string
	Group                   string
	DataID                  string
	Version                 uint64
	ExpectedPreviousVersion uint64
	ValueDigest             string
	ValueRef                string
	Sensitive               bool
	CreatedBy               string
	ApprovedBy              string
	ApprovalID              string
	State                   State
	UpdatedAt               time.Time
}

type Request struct {
	Action     Action
	At         time.Time
	ActorID    string
	ApprovalID string
}

type AuditEntry struct {
	ChangeID    string
	Action      Action
	Before      State
	After       State
	ActorID     string
	ApprovalID  string
	At          time.Time
	ValueDigest string
}

type Auditor interface {
	Record(context.Context, AuditEntry) error
}

func New(id, organizationID, namespace, group, dataID, valueDigest, valueRef, creator string, version, expectedPreviousVersion uint64, sensitive bool, at time.Time) (Change, error) {
	change := Change{
		ID: id, OrganizationID: organizationID, Namespace: namespace, Group: group, DataID: dataID,
		Version: version, ExpectedPreviousVersion: expectedPreviousVersion, ValueDigest: valueDigest,
		ValueRef: valueRef, Sensitive: sensitive, CreatedBy: creator, State: StatePendingApproval, UpdatedAt: at.UTC(),
	}
	if err := Validate(change); err != nil {
		return Change{}, err
	}
	return change, nil
}

func Validate(change Change) error {
	for label, value := range map[string]string{
		"change id": change.ID, "organization id": change.OrganizationID, "namespace": change.Namespace,
		"group": change.Group, "data id": change.DataID, "creator": change.CreatedBy, "value reference": change.ValueRef,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("config %s is required", label)
		}
	}
	if change.Version == 0 || change.Version > math.MaxInt64 || change.ExpectedPreviousVersion > math.MaxInt64 || change.Version <= change.ExpectedPreviousVersion {
		return errors.New("config version must be greater than the expected previous version")
	}
	digest := strings.TrimSpace(change.ValueDigest)
	if len(digest) != 64 {
		return errors.New("config value digest must be a sha256 digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return errors.New("config value digest is not hexadecimal")
	}
	if change.UpdatedAt.IsZero() {
		return errors.New("config change timestamp is required")
	}
	return nil
}

func Apply(change Change, request Request) (Change, AuditEntry, error) {
	if err := Validate(change); err != nil {
		return Change{}, AuditEntry{}, err
	}
	if strings.TrimSpace(request.ActorID) == "" || request.At.IsZero() || strings.TrimSpace(request.ApprovalID) == "" {
		return Change{}, AuditEntry{}, errors.New("config transition actor, timestamp, and approval are required")
	}
	next := change
	next.UpdatedAt = request.At.UTC()
	next.ApprovalID = strings.TrimSpace(request.ApprovalID)
	switch request.Action {
	case ActionApprove:
		if change.State != StatePendingApproval || request.ActorID == change.CreatedBy {
			return Change{}, AuditEntry{}, errors.New("config change requires a different approver")
		}
		next.State = StateApproved
		next.ApprovedBy = request.ActorID
	case ActionPublish:
		if change.State != StateApproved || request.ActorID != change.ApprovedBy {
			return Change{}, AuditEntry{}, errors.New("only the recorded approver can publish this config change")
		}
		next.State = StatePublished
	case ActionRequestRollback:
		if change.State != StatePublished {
			return Change{}, AuditEntry{}, transitionError(change.State, request.Action)
		}
		next.State = StateRollbackPending
	case ActionRollback:
		if change.State != StateRollbackPending || request.ActorID == change.CreatedBy {
			return Change{}, AuditEntry{}, errors.New("config rollback requires an independent operator")
		}
		next.State = StateRolledBack
	case ActionReject:
		if change.State != StatePendingApproval || request.ActorID == change.CreatedBy {
			return Change{}, AuditEntry{}, transitionError(change.State, request.Action)
		}
		next.State = StateRejected
	default:
		return Change{}, AuditEntry{}, fmt.Errorf("unsupported config action %q", request.Action)
	}
	return next, AuditEntry{ChangeID: change.ID, Action: request.Action, Before: change.State, After: next.State, ActorID: request.ActorID, ApprovalID: request.ApprovalID, At: request.At.UTC(), ValueDigest: change.ValueDigest}, nil
}

func ApplyWithAudit(ctx context.Context, auditor Auditor, change Change, request Request) (Change, error) {
	if auditor == nil {
		return Change{}, errors.New("config transition auditor is required")
	}
	next, entry, err := Apply(change, request)
	if err != nil {
		return Change{}, err
	}
	if err := auditor.Record(ctx, entry); err != nil {
		return Change{}, fmt.Errorf("record config transition audit: %w", err)
	}
	return next, nil
}

func transitionError(state State, action Action) error {
	return fmt.Errorf("config action %q is not allowed from state %q", action, state)
}
