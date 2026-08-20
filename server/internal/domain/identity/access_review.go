package identity

import "time"

const (
	AccessReviewOpen      = "OPEN"
	AccessReviewCompleted = "COMPLETED"
	AccessReviewExpired   = "EXPIRED"
	AccessReviewApprove   = "APPROVE"
	AccessReviewRevoke    = "REVOKE"
	AccessReviewException = "EXCEPTION"
)

type AccessReview struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	ReviewerID     string     `json:"reviewer_id"`
	ReviewerName   string     `json:"reviewer_name"`
	Status         string     `json:"status"`
	DueAt          time.Time  `json:"due_at"`
	CreatedBy      string     `json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type AccessReviewItem struct {
	ID             string     `json:"id"`
	ReviewID       string     `json:"review_id"`
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id"`
	LoginName      string     `json:"login_name"`
	RoleKey        string     `json:"role_key"`
	Decision       string     `json:"decision"`
	Reason         string     `json:"reason"`
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
