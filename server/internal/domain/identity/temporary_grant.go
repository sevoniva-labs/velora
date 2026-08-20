package identity

import "time"

type TemporaryRoleGrant struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id"`
	RoleKey        string     `json:"role_key"`
	RequestedBy    string     `json:"requested_by"`
	ApprovalID     string     `json:"approval_id"`
	Reason         string     `json:"reason"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidUntil     time.Time  `json:"valid_until"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	RevokedBy      string     `json:"revoked_by,omitempty"`
	RevokeReason   string     `json:"revoke_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
