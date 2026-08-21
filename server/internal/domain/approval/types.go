package approval

import (
	"errors"
	"strings"
	"time"
)

const (
	ModeAny    = "ANY"
	ModeAll    = "ALL"
	ModeQuorum = "QUORUM"

	StatusPending   = "PENDING"
	StatusApproved  = "APPROVED"
	StatusRejected  = "REJECTED"
	StatusWithdrawn = "WITHDRAWN"
	StatusExpired   = "EXPIRED"

	DecisionApprove = "APPROVE"
	DecisionReject  = "REJECT"
)

var ErrInvalidRequest = errors.New("invalid approval request")
var ErrMakerChecker = errors.New("maker cannot approve own request")
var ErrNotPending = errors.New("approval request is not pending")
var ErrTaskNotAssigned = errors.New("approval task is not assigned to actor")
var ErrDigestMismatch = errors.New("approval request digest does not match command")
var ErrAlreadyExecuted = errors.New("approval request was already executed")

type Request struct {
	ID                string
	OrganizationID    string
	RequestType       string
	Action            string
	Resource          string
	ResourceID        string
	Summary           string
	PayloadJSON       string
	RequestDigest     string
	ApplicantID       string
	Mode              string
	RequiredApprovals int
	Status            string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Tasks             []Task
}

type Task struct {
	ID              string
	RequestID       string
	AssigneeID      string
	Status          string
	Decision        string
	Comment         string
	TransferredFrom string
	DecidedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func ValidateCreation(request Request, approverIDs []string) error {
	request.Mode = strings.ToUpper(strings.TrimSpace(request.Mode))
	if request.OrganizationID == "" || request.ApplicantID == "" || request.RequestType == "" || request.Action == "" || request.Resource == "" || request.RequestDigest == "" || request.Summary == "" || len(approverIDs) == 0 || !request.ExpiresAt.After(time.Now().UTC()) {
		return ErrInvalidRequest
	}
	seen := make(map[string]struct{}, len(approverIDs))
	for _, approverID := range approverIDs {
		if approverID == "" || approverID == request.ApplicantID {
			return ErrMakerChecker
		}
		if _, exists := seen[approverID]; exists {
			return ErrInvalidRequest
		}
		seen[approverID] = struct{}{}
	}
	switch request.Mode {
	case ModeAny:
		if request.RequiredApprovals != 1 {
			return ErrInvalidRequest
		}
	case ModeAll:
		if request.RequiredApprovals != len(approverIDs) {
			return ErrInvalidRequest
		}
	case ModeQuorum:
		if request.RequiredApprovals < 1 || request.RequiredApprovals > len(approverIDs) {
			return ErrInvalidRequest
		}
	default:
		return ErrInvalidRequest
	}
	return nil
}

func ResolveStatus(required, approved, rejected, pending int) string {
	if rejected > 0 || approved+pending < required {
		return StatusRejected
	}
	if approved >= required {
		return StatusApproved
	}
	return StatusPending
}
