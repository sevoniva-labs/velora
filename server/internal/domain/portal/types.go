package portal

import (
	"net/url"
	"sort"
	"strings"
	"time"

	identity "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

const (
	StatusActive   = "ACTIVE"
	StatusEnabled  = "ENABLED"
	StatusDisabled = "DISABLED"

	PolicyEveryone              = "EVERYONE"
	PolicyUser                  = "USER"
	PolicyRole                  = "ROLE"
	PolicyGroup                 = "GROUP"
	PolicyOrganization          = "ORGANIZATION"
	LaunchTypeRetiredVeloraOIDC = "VELORA_OIDC"
)

type Category struct {
	ID             string
	OrganizationID string
	Key            string
	Name           string
	Description    string
	Status         string
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Tag struct {
	ID             string
	OrganizationID string
	Key            string
	Name           string
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AccessPolicy struct {
	ID            string
	ApplicationID string
	Type          string
	Value         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const (
	AccessSubjectEveryone     = "EVERYONE"
	AccessSubjectDepartment   = "DEPARTMENT"
	AccessSubjectUserGroup    = "USER_GROUP"
	AccessSubjectPlatformRole = "PLATFORM_ROLE"
	AccessSubjectUser         = "USER"
	AccessEffectAllow         = "ALLOW"
	AccessEffectExclude       = "EXCLUDE"
)

type AccessGrant struct {
	ID, OrganizationID, ApplicationID string
	SubjectType, SubjectID            string
	SubjectName                       string
	IncludeDescendants                bool
	Effect, Status, Reason            string
	Roles                             []string
	ValidFrom, ValidUntil             *time.Time
	Version                           int64
	CreatedBy, UpdatedBy              string
	CreatedAt, UpdatedAt              time.Time
}

type AccessSubjectProfile struct {
	UserID, LoginName, DisplayName string
	Roles, GroupIDs, DepartmentIDs []string
}

type EffectiveAccess struct {
	UserID, LoginName, DisplayName string
	Allowed                        bool
	Roles, SourceGrantIDs          []string
}

type EffectiveApplicationAccessSource struct {
	GrantID, SubjectType, SubjectID, SubjectName, Effect string
}

type UserEffectiveApplicationAccess struct {
	UserID, ApplicationID, ApplicationCode, ApplicationName, Status string
	Roles                                                           []string
	Sources                                                         []EffectiveApplicationAccessSource
}

func ResolveAccessGrants(grants []AccessGrant, profile AccessSubjectProfile, departmentParents map[string]string, now time.Time) EffectiveAccess {
	result := EffectiveAccess{UserID: profile.UserID, LoginName: profile.LoginName, DisplayName: profile.DisplayName}
	roleSet := make(map[string]struct{})
	sourceSet := make(map[string]struct{})
	excluded := false
	for _, grant := range grants {
		if !grantActive(grant, now) || !grantMatches(grant, profile, departmentParents) {
			continue
		}
		if grant.Effect == AccessEffectExclude {
			excluded = true
			continue
		}
		result.Allowed = true
		sourceSet[grant.ID] = struct{}{}
		for _, role := range grant.Roles {
			if role = strings.TrimSpace(role); role != "" {
				roleSet[role] = struct{}{}
			}
		}
	}
	if excluded {
		result.Allowed = false
		return result
	}
	for role := range roleSet {
		result.Roles = append(result.Roles, role)
	}
	for source := range sourceSet {
		result.SourceGrantIDs = append(result.SourceGrantIDs, source)
	}
	sort.Strings(result.Roles)
	sort.Strings(result.SourceGrantIDs)
	return result
}

func grantActive(grant AccessGrant, now time.Time) bool {
	if grant.Status != StatusActive {
		return false
	}
	if grant.ValidFrom != nil && now.Before(*grant.ValidFrom) {
		return false
	}
	return grant.ValidUntil == nil || now.Before(*grant.ValidUntil)
}

func grantMatches(grant AccessGrant, profile AccessSubjectProfile, departmentParents map[string]string) bool {
	switch grant.SubjectType {
	case AccessSubjectEveryone:
		return true
	case AccessSubjectUser:
		return grant.SubjectID == profile.UserID
	case AccessSubjectUserGroup:
		return contains(profile.GroupIDs, grant.SubjectID)
	case AccessSubjectPlatformRole:
		return contains(profile.Roles, grant.SubjectID)
	case AccessSubjectDepartment:
		for _, departmentID := range profile.DepartmentIDs {
			if departmentID == grant.SubjectID || grant.IncludeDescendants && departmentDescendsFrom(departmentID, grant.SubjectID, departmentParents) {
				return true
			}
		}
	}
	return false
}

func departmentDescendsFrom(departmentID, ancestorID string, parents map[string]string) bool {
	seen := make(map[string]struct{})
	for current := parents[departmentID]; current != ""; current = parents[current] {
		if current == ancestorID {
			return true
		}
		if _, exists := seen[current]; exists {
			return false
		}
		seen[current] = struct{}{}
	}
	return false
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

const (
	RoleRiskNormal     = "NORMAL"
	RoleRiskPrivileged = "PRIVILEGED"
	RoleRiskCritical   = "CRITICAL"
)

type ApplicationRole struct {
	ID             string
	OrganizationID string
	ApplicationID  string
	Key            string
	Name           string
	Description    string
	RiskLevel      string
	Status         string
	ConfigVersion  int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProvisioningTarget struct {
	ID                 string
	OrganizationID     string
	ApplicationID      string
	EndpointURL        string
	SigningAlgorithm   string
	SecretRef          string
	SecretFingerprint  string
	ActiveKeyVersion   int64
	PreviousKeyVersion *int64
	PreviousValidUntil *time.Time
	DeliveryStatus     string
	LastSuccessAt      *time.Time
	LastFailureAt      *time.Time
	LastErrorCode      string
	ConfigVersion      int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type OnboardingCheck struct {
	ID             string
	OrganizationID string
	ApplicationID  string
	ConfigVersion  int64
	CheckType      string
	Result         string
	ErrorCode      string
	EvidenceJSON   string
	RequestID      string
	VerifiedBy     string
	OccurredAt     time.Time
}

func OnboardingChecksPassed(items []OnboardingCheck) bool {
	required := map[string]bool{"access_policy": false, "oidc_discovery": false, "provisioning_challenge": false, "provisioning_duplicate": false, "provisioning_stale": false}
	for _, item := range items {
		if _, ok := required[item.CheckType]; ok && item.Result == "PASSED" {
			required[item.CheckType] = true
		}
	}
	for _, passed := range required {
		if !passed {
			return false
		}
	}
	return true
}

type Application struct {
	ID                  string
	OrganizationID      string
	Code                string
	Name                string
	Description         string
	Icon                string
	CategoryID          string
	CategoryName        string
	OwnerUserID         string
	OwnerUserName       string
	OwnerDepartmentID   string
	OwnerDepartmentName string
	HomeURL             string
	LaunchURL           string
	LaunchType          string
	Status              string
	SortOrder           int
	Featured            bool
	Favorite            bool
	VisitCount          int64
	Tags                []Tag
	Policies            []AccessPolicy
	CreatedBy           string
	UpdatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LifecycleStatus     string
	PublishedAt         *time.Time
	PublishedBy         string
	ConfigVersion       int64
}

type AccessContext struct {
	Principal identity.Principal
	Groups    []string
}

func CanAccess(app Application, ctx AccessContext) bool {
	if app.OrganizationID == "" || ctx.Principal.OrganizationID != app.OrganizationID || app.Status != StatusEnabled {
		return false
	}
	if app.LifecycleStatus != "" && app.LifecycleStatus != LifecyclePublished {
		return false
	}
	if len(app.Policies) == 0 {
		return false
	}
	for _, policy := range app.Policies {
		switch strings.ToUpper(strings.TrimSpace(policy.Type)) {
		case PolicyEveryone:
			return true
		case PolicyOrganization:
			if strings.TrimSpace(policy.Value) == app.OrganizationID {
				return true
			}
		case PolicyUser:
			if strings.TrimSpace(policy.Value) == ctx.Principal.UserID {
				return true
			}
		case PolicyRole:
			if ctx.Principal.HasRole(strings.TrimSpace(policy.Value)) {
				return true
			}
		case PolicyGroup:
			for _, group := range ctx.Groups {
				if group == strings.TrimSpace(policy.Value) {
					return true
				}
			}
		}
	}
	return false
}

func ValidateApplication(app Application) error {
	if strings.TrimSpace(app.Code) == "" || strings.TrimSpace(app.Name) == "" {
		return ErrInvalidApplication
	}
	if app.Status != StatusEnabled && app.Status != StatusDisabled {
		return ErrInvalidApplication
	}
	if strings.EqualFold(strings.TrimSpace(app.LaunchType), LaunchTypeRetiredVeloraOIDC) {
		return ErrRetiredLaunchType
	}
	if app.LaunchType == "" {
		app.LaunchType = "URL"
	}
	if app.HomeURL != "" {
		u, err := url.Parse(app.HomeURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return ErrInvalidLaunchURL
		}
	}
	if app.LaunchURL != "" {
		u, err := url.Parse(app.LaunchURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return ErrInvalidLaunchURL
		}
	}
	return nil
}

var (
	ErrInvalidApplication = &ValidationError{Message: "invalid portal application"}
	ErrInvalidLaunchURL   = &ValidationError{Message: "launch URL must be an absolute HTTPS URL"}
	ErrRetiredLaunchType  = &ValidationError{Message: "Velora OIDC provider is retired; migrate this application to Casdoor OIDC or another approved integration"}
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
