package portal

import "time"

const (
	OperationPending        = "PENDING"
	OperationRunning        = "RUNNING"
	OperationSucceeded      = "SUCCEEDED"
	OperationFailed         = "FAILED"
	OperationActionRequired = "ACTION_REQUIRED"
)

type OnboardingOperation struct {
	ID, OrganizationID, ApplicationID string
	OperationType, Status             string
	DesiredVersion                    int64
	IdempotencyKey                    string
	AttemptCount                      int
	ProviderRequestID                 string
	ResultSummaryJSON                 string
	ErrorCode                         string
	NextRetryAt, CompletedAt          *time.Time
	CreatedAt, UpdatedAt              time.Time
}
