package jobcontrol

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type State string

const (
	StatePending             State = "PENDING"
	StateRunning             State = "RUNNING"
	StateSucceeded           State = "SUCCEEDED"
	StateFailed              State = "FAILED"
	StateDeadLetter          State = "DEAD_LETTER"
	StateCompensationPending State = "COMPENSATION_PENDING"
	StateCompensating        State = "COMPENSATING"
	StateCompensated         State = "COMPENSATED"
	StateRollbackPending     State = "ROLLBACK_PENDING"
	StateRollingBack         State = "ROLLING_BACK"
	StateRolledBack          State = "ROLLED_BACK"
	StateCancelled           State = "CANCELLED"
)

type Action string

const (
	ActionStart                Action = "START"
	ActionSucceed              Action = "SUCCEED"
	ActionFail                 Action = "FAIL"
	ActionRetry                Action = "RETRY"
	ActionRequestCompensation  Action = "REQUEST_COMPENSATION"
	ActionStartCompensation    Action = "START_COMPENSATION"
	ActionCompleteCompensation Action = "COMPLETE_COMPENSATION"
	ActionRequestRollback      Action = "REQUEST_ROLLBACK"
	ActionStartRollback        Action = "START_ROLLBACK"
	ActionCompleteRollback     Action = "COMPLETE_ROLLBACK"
	ActionCancel               Action = "CANCEL"
)

type Definition struct {
	Name                 string
	MaxAttempts          int
	RetryBaseDelay       time.Duration
	RetryMaxDelay        time.Duration
	RequiresApproval     bool
	RequiresCompensation bool
}

type Run struct {
	ID             string
	JobName        string
	OrganizationID string
	IdempotencyKey string
	State          State
	Attempt        int
	MaxAttempts    int
	NextAttemptAt  time.Time
	ApprovalID     string
	RequestedBy    string
	LastError      string
	UpdatedAt      time.Time
}

type Request struct {
	Action     Action
	At         time.Time
	ActorID    string
	ApprovalID string
	Cause      string
}

type AuditEntry struct {
	RunID          string
	JobName        string
	Action         Action
	Before         State
	After          State
	ActorID        string
	ApprovalID     string
	At             time.Time
	IdempotencyKey string
	Cause          string
}

type Auditor interface {
	Record(context.Context, AuditEntry) error
}

func ValidateDefinition(def Definition) error {
	if strings.TrimSpace(def.Name) == "" {
		return errors.New("job definition name is required")
	}
	if def.MaxAttempts < 1 || def.MaxAttempts > 100 {
		return errors.New("job max attempts must be between 1 and 100")
	}
	if def.RetryBaseDelay < time.Second || def.RetryBaseDelay > 24*time.Hour {
		return errors.New("job retry base delay must be between 1 second and 24 hours")
	}
	if def.RetryMaxDelay < def.RetryBaseDelay || def.RetryMaxDelay > 7*24*time.Hour {
		return errors.New("job retry max delay must be at least the base delay and no more than 7 days")
	}
	return nil
}

func ValidateRun(def Definition, run Run) error {
	if err := ValidateDefinition(def); err != nil {
		return err
	}
	if strings.TrimSpace(run.ID) == "" || strings.TrimSpace(run.JobName) == "" || strings.TrimSpace(run.IdempotencyKey) == "" {
		return errors.New("job run id, job name, and idempotency key are required")
	}
	if run.JobName != def.Name {
		return errors.New("job run definition mismatch")
	}
	if run.MaxAttempts != def.MaxAttempts {
		return errors.New("job run max attempts must match the definition")
	}
	if run.Attempt < 0 || run.Attempt > run.MaxAttempts {
		return errors.New("job run attempt is outside the definition limit")
	}
	return nil
}

func Apply(def Definition, run Run, req Request) (Run, AuditEntry, error) {
	if err := ValidateRun(def, run); err != nil {
		return Run{}, AuditEntry{}, err
	}
	if strings.TrimSpace(req.ActorID) == "" || req.At.IsZero() {
		return Run{}, AuditEntry{}, errors.New("job transition actor and timestamp are required")
	}
	if requiresApproval(req.Action) && strings.TrimSpace(req.ApprovalID) == "" {
		return Run{}, AuditEntry{}, errors.New("job transition approval is required")
	}
	if def.RequiresApproval && req.Action == ActionStart && strings.TrimSpace(req.ApprovalID) == "" {
		return Run{}, AuditEntry{}, errors.New("job start approval is required")
	}

	next := run
	next.UpdatedAt = req.At.UTC()
	next.ApprovalID = strings.TrimSpace(req.ApprovalID)
	next.RequestedBy = req.ActorID
	if req.Cause != "" {
		next.LastError = truncate(req.Cause, 1000)
	}
	switch req.Action {
	case ActionStart:
		if run.State != StatePending {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateRunning
		next.Attempt = 1
	case ActionSucceed:
		if run.State != StateRunning {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateSucceeded
		next.LastError = ""
	case ActionFail:
		if run.State != StateRunning {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		if strings.TrimSpace(req.Cause) == "" {
			return Run{}, AuditEntry{}, errors.New("failed job transition requires a cause")
		}
		if run.Attempt >= run.MaxAttempts {
			next.State = StateDeadLetter
			next.NextAttemptAt = time.Time{}
		} else {
			next.State = StateFailed
			next.NextAttemptAt = req.At.UTC().Add(retryDelay(def, run.Attempt))
		}
	case ActionRetry:
		if run.State != StateFailed || run.Attempt >= run.MaxAttempts {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		if req.At.Before(run.NextAttemptAt) {
			return Run{}, AuditEntry{}, errors.New("job retry backoff has not elapsed")
		}
		next.State = StateRunning
		next.Attempt++
		next.NextAttemptAt = time.Time{}
	case ActionRequestCompensation:
		if !def.RequiresCompensation || (run.State != StateFailed && run.State != StateDeadLetter) {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateCompensationPending
	case ActionStartCompensation:
		if run.State != StateCompensationPending {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateCompensating
	case ActionCompleteCompensation:
		if run.State != StateCompensating {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateCompensated
	case ActionRequestRollback:
		if run.State != StateSucceeded {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateRollbackPending
	case ActionStartRollback:
		if run.State != StateRollbackPending {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateRollingBack
	case ActionCompleteRollback:
		if run.State != StateRollingBack {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateRolledBack
	case ActionCancel:
		if run.State != StatePending && run.State != StateFailed && run.State != StateDeadLetter {
			return Run{}, AuditEntry{}, invalidTransition(run.State, req.Action)
		}
		next.State = StateCancelled
	default:
		return Run{}, AuditEntry{}, fmt.Errorf("unsupported job action %q", req.Action)
	}

	return next, AuditEntry{
		RunID: run.ID, JobName: run.JobName, Action: req.Action, Before: run.State, After: next.State,
		ActorID: req.ActorID, ApprovalID: strings.TrimSpace(req.ApprovalID), At: req.At.UTC(),
		IdempotencyKey: run.IdempotencyKey, Cause: truncate(req.Cause, 1000),
	}, nil
}

func ApplyWithAudit(ctx context.Context, auditor Auditor, def Definition, run Run, req Request) (Run, error) {
	if auditor == nil {
		return Run{}, errors.New("job transition auditor is required")
	}
	next, entry, err := Apply(def, run, req)
	if err != nil {
		return Run{}, err
	}
	if err := auditor.Record(ctx, entry); err != nil {
		return Run{}, fmt.Errorf("record job transition audit: %w", err)
	}
	return next, nil
}

func requiresApproval(action Action) bool {
	return action == ActionRequestCompensation || action == ActionStartCompensation ||
		action == ActionRequestRollback || action == ActionStartRollback ||
		action == ActionCompleteRollback || action == ActionCancel
}

func retryDelay(def Definition, attempt int) time.Duration {
	delay := def.RetryBaseDelay
	for i := 1; i < attempt; i++ {
		if delay >= def.RetryMaxDelay/2 {
			return def.RetryMaxDelay
		}
		delay *= 2
	}
	if delay > def.RetryMaxDelay {
		return def.RetryMaxDelay
	}
	return delay
}

func invalidTransition(state State, action Action) error {
	return fmt.Errorf("job action %q is not allowed from state %q", action, state)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
