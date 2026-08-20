package portal

import (
	"net/url"
	"strings"
	"time"

	identity "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

const (
	StatusActive   = "ACTIVE"
	StatusEnabled  = "ENABLED"
	StatusDisabled = "DISABLED"

	PolicyEveryone     = "EVERYONE"
	PolicyUser         = "USER"
	PolicyRole         = "ROLE"
	PolicyGroup        = "GROUP"
	PolicyOrganization = "ORGANIZATION"
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

type Application struct {
	ID             string
	OrganizationID string
	Code           string
	Name           string
	Description    string
	Icon           string
	CategoryID     string
	CategoryName   string
	HomeURL        string
	LaunchURL      string
	LaunchType     string
	Status         string
	SortOrder      int
	Featured       bool
	Favorite       bool
	VisitCount     int64
	Tags           []Tag
	Policies       []AccessPolicy
	CreatedBy      string
	UpdatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AccessContext struct {
	Principal identity.Principal
	Groups    []string
}

func CanAccess(app Application, ctx AccessContext) bool {
	if app.OrganizationID == "" || ctx.Principal.OrganizationID != app.OrganizationID || app.Status != StatusEnabled {
		return false
	}
	if len(app.Policies) == 0 {
		return true
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
	if app.LaunchType == "" {
		app.LaunchType = "URL"
	}
	if app.LaunchURL != "" {
		u, err := url.Parse(app.LaunchURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return ErrInvalidLaunchURL
		}
	}
	return nil
}

var (
	ErrInvalidApplication = &ValidationError{Message: "invalid portal application"}
	ErrInvalidLaunchURL   = &ValidationError{Message: "launch URL must be an absolute HTTPS URL"}
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
