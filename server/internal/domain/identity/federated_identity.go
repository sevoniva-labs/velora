package identity

import "time"

// FederatedIdentityLink is an explicit, administrator-controlled mapping from
// an external identity to a local account. Provider subjects are never
// auto-linked by email, login name, or display name.
type FederatedIdentityLink struct {
	ID                  string     `json:"id"`
	OrganizationID      string     `json:"organization_id"`
	Provider            string     `json:"provider"`
	Subject             string     `json:"subject"`
	UserID              string     `json:"user_id"`
	LoginName           string     `json:"login_name"`
	CreatedBy           string     `json:"created_by"`
	ApprovalID          string     `json:"approval_id"`
	CreatedAt           time.Time  `json:"created_at"`
	LastAuthenticatedAt *time.Time `json:"last_authenticated_at,omitempty"`
}
