package identity

import "time"

const (
	DataScopeOrganization   = "ORGANIZATION"
	DataScopeDepartment     = "DEPARTMENT"
	DataScopeDepartmentTree = "DEPARTMENT_TREE"
	DataScopeSelf           = "SELF"
	DataScopeCustom         = "CUSTOM"
)

type EffectiveDataScope struct {
	OrganizationWide bool     `json:"organization_wide"`
	Self             bool     `json:"self"`
	DepartmentIDs    []string `json:"department_ids"`
}

type MFAFactor struct {
	UserID           string
	Status           string
	SecretCiphertext string
	KeyVersion       string
	PendingExpiresAt time.Time
	ConfirmedAt      *time.Time
}

type MFAEnrollment struct {
	Secret string
	URL    string
}

func (scope EffectiveDataScope) Allows(userID, departmentID, actorUserID string) bool {
	if scope.OrganizationWide {
		return true
	}
	if scope.Self && userID != "" && userID == actorUserID {
		return true
	}
	for _, allowed := range scope.DepartmentIDs {
		if allowed == departmentID {
			return true
		}
	}
	return false
}

type Organization struct {
	ID          string    `json:"id"`
	Key         string    `json:"org_key"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	MaxUsers    int       `json:"max_users"`
	MaxSessions int       `json:"max_active_sessions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Department struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	ParentID       string    `json:"parent_id,omitempty"`
	Key            string    `json:"department_key"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Position struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	DepartmentID   string    `json:"department_id"`
	Key            string    `json:"position_key"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserGroup struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Key            string    `json:"group_key"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	Roles          []string  `json:"roles"`
	MemberIDs      []string  `json:"member_ids"`
	MemberCount    int       `json:"member_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserAssignment struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id"`
	DepartmentID   string     `json:"department_id"`
	PositionID     string     `json:"position_id,omitempty"`
	Primary        bool       `json:"primary"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidUntil     *time.Time `json:"valid_until,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type SecurityPolicy struct {
	PasswordMinLength        int   `json:"password_min_length"`
	PasswordRequireUpper     bool  `json:"password_require_upper"`
	PasswordRequireLower     bool  `json:"password_require_lower"`
	PasswordRequireDigit     bool  `json:"password_require_digit"`
	PasswordRequireSymbol    bool  `json:"password_require_symbol"`
	PasswordHistory          int   `json:"password_history"`
	PasswordMaxAgeDays       int   `json:"password_max_age_days"`
	LoginMaxFailures         int   `json:"login_max_failures"`
	LoginLockDurationSeconds int64 `json:"login_lock_duration_seconds"`
	SessionTTLSeconds        int64 `json:"session_ttl_seconds"`
	MaxConcurrentSessions    int   `json:"max_active_sessions"`
}

const (
	SecuritySettingPasswordMinLength     = "security.password_min_length"
	SecuritySettingPasswordRequireUpper  = "security.password_require_upper"
	SecuritySettingPasswordRequireLower  = "security.password_require_lower"
	SecuritySettingPasswordRequireDigit  = "security.password_require_digit"
	SecuritySettingPasswordRequireSymbol = "security.password_require_symbol"
	SecuritySettingPasswordHistory       = "security.password_history"
	SecuritySettingPasswordMaxAgeDays    = "security.password_max_age_days"
	SecuritySettingLoginMaxFailures      = "security.login_max_failures"
	SecuritySettingLoginLockDurationSec  = "security.login_lock_duration_seconds"
	SecuritySettingSessionTTLSeconds     = "security.session_ttl_seconds"
	SecuritySettingMaxConcurrentSessions = "security.max_active_sessions"
)

type Permission struct {
	ID        string    `json:"id"`
	Key       string    `json:"permission_key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Menu struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Key            string    `json:"menu_key"`
	ParentKey      string    `json:"parent_key"`
	Name           string    `json:"name"`
	Route          string    `json:"route"`
	Icon           string    `json:"icon"`
	PermissionKey  string    `json:"permission_key"`
	SortOrder      int       `json:"sort_order"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Role struct {
	ID          string       `json:"id"`
	Key         string       `json:"role_key"`
	Name        string       `json:"name"`
	DataScope   string       `json:"data_scope"`
	Departments []string     `json:"data_scope_departments"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
}

type RoleConflictRule struct {
	ID             string
	OrganizationID string
	RoleA          string
	RoleB          string
	Reason         string
}

type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	LoginName   string    `json:"login_name"`
	DisplayName string    `json:"display_name"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ClientIP    string    `json:"client_ip"`
	UserAgent   string    `json:"user_agent"`
	Current     bool      `json:"current"`
}

type User struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organization_id"`
	LoginName          string     `json:"login_name"`
	DisplayName        string     `json:"display_name"`
	Status             string     `json:"status"`
	MustChangePassword bool       `json:"must_change_password"`
	FailedLoginCount   int        `json:"-"`
	LockedUntil        *time.Time `json:"locked_until,omitempty"`
	PasswordChangedAt  time.Time  `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Roles              []string   `json:"roles"`
	Permissions        []string   `json:"permissions,omitempty"`
}

func (u User) HasRole(keys ...string) bool {
	for _, have := range u.Roles {
		for _, want := range keys {
			if have == want {
				return true
			}
		}
	}
	return false
}

type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Principal struct {
	Type                string             `json:"principal_type"`
	UserID              string             `json:"user_id"`
	OrganizationID      string             `json:"organization_id"`
	LoginName           string             `json:"login_name"`
	DisplayName         string             `json:"display_name"`
	Roles               []string           `json:"roles"`
	Permissions         []string           `json:"permissions,omitempty"`
	Scopes              []string           `json:"scopes,omitempty"`
	MustChangePassword  bool               `json:"must_change_password"`
	SessionID           string             `json:"-"`
	PasswordChangedAt   time.Time          `json:"-"`
	DataScope           EffectiveDataScope `json:"data_scope"`
	AuthenticationLevel string             `json:"authentication_level"`
	MFAVerifiedAt       *time.Time         `json:"mfa_verified_at,omitempty"`
}

func (p Principal) HasRole(keys ...string) bool {
	for _, have := range p.Roles {
		for _, want := range keys {
			if have == want {
				return true
			}
		}
	}
	return false
}
func (p Principal) HasPermission(keys ...string) bool {
	if p.Type == "TOKEN" {
		for _, want := range keys {
			allowed := false
			for _, scope := range p.Scopes {
				if scope == want || scope == "*" {
					allowed = true
					break
				}
			}
			if !allowed {
				return false
			}
		}
	}
	if p.HasRole("system_admin") {
		return true
	}
	for _, have := range p.Permissions {
		for _, want := range keys {
			if have == want {
				return true
			}
		}
	}
	return false
}
