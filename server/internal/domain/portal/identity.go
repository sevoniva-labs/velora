package portal

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	LifecycleDraft               = "DRAFT"
	LifecycleIdentityPending     = "IDENTITY_PENDING"
	LifecycleVerificationPending = "VERIFICATION_PENDING"
	LifecycleReady               = "READY"
	LifecyclePublished           = "PUBLISHED"
	LifecycleDisabled            = "DISABLED"
	LifecycleVerificationFailed  = "VERIFICATION_FAILED"

	IdentityProviderCasdoor = "casdoor"
	ProtocolOIDC            = "OIDC"
	ProtocolSAML            = "SAML"
	ProtocolCAS             = "CAS"
	ProtocolForwardAuth     = "FORWARD_AUTH"
	VerificationPassed      = "PASSED"
	VerificationFailed      = "FAILED"
)

var DefaultOIDCScopes = []string{"openid", "profile", "email"}

var (
	ErrIdentityBindingRequired = errors.New("identity binding is required")
	ErrOptimisticConflict      = errors.New("configuration was changed by another operator")
	ErrPublishNotReady         = errors.New("application is not ready for publish")
	ErrInvalidIdentityBinding  = errors.New("invalid identity binding")
)

type IdentityBinding struct {
	ID                     string
	OrganizationID         string
	ApplicationID          string
	ProviderKey            string
	Protocol               string
	ProviderApplicationRef string
	PublicClientID         string
	Issuer                 string
	RedirectURIs           []string
	Environments           []ApplicationEnvironment
	Scopes                 []string
	ConfigurationStatus    string
	VerificationStatus     string
	VerifiedAt             *time.Time
	VerifiedBy             string
	VerificationError      string
	ConfigVersion          int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type IdentityBindingInput struct {
	ProviderKey            string
	Protocol               string
	ProviderApplicationRef string
	PublicClientID         string
	Issuer                 string
	RedirectURIs           []string
	Environments           []ApplicationEnvironment
	Scopes                 []string
	ConfigurationStatus    string
	VerificationStatus     string
}

type ApplicationEnvironment struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type Verification struct {
	ID             string
	OrganizationID string
	ApplicationID  string
	BindingID      string
	CheckType      string
	Result         string
	ErrorCode      string
	Evidence       string
	VerifiedBy     string
	OccurredAt     time.Time
	RequestID      string
}

func (b IdentityBindingInput) Validate() error {
	if strings.ToLower(strings.TrimSpace(b.ProviderKey)) != IdentityProviderCasdoor {
		return ErrInvalidIdentityBinding
	}
	// Only the OIDC path is part of the verified Phase 0-4 contract. Legacy
	// SAML/CAS/ForwardAuth records remain readable for migration, but cannot be
	// newly saved or published until their own verified integration is designed.
	if strings.ToUpper(strings.TrimSpace(b.Protocol)) != ProtocolOIDC {
		return ErrInvalidIdentityBinding
	}
	if strings.TrimSpace(b.ProviderApplicationRef) == "" || len(b.ProviderApplicationRef) > 255 {
		return ErrInvalidIdentityBinding
	}
	if strings.TrimSpace(b.PublicClientID) == "" || len(b.PublicClientID) > 255 {
		return ErrInvalidIdentityBinding
	}
	if strings.TrimSpace(b.Issuer) != "" {
		u, err := url.Parse(strings.TrimSpace(b.Issuer))
		if err != nil || !validIdentityURL(u) {
			return ErrInvalidIdentityBinding
		}
	}
	if len(b.RedirectURIs) > 32 {
		return ErrInvalidIdentityBinding
	}
	if err := validateApplicationEnvironments(b.Environments, b.RedirectURIs); err != nil {
		return err
	}
	for _, raw := range b.RedirectURIs {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || !validIdentityURL(u) {
			return ErrInvalidIdentityBinding
		}
	}
	if _, err := NormalizeOIDCScopes(b.Scopes); err != nil {
		return ErrInvalidIdentityBinding
	}
	return nil
}

func validateApplicationEnvironments(environments []ApplicationEnvironment, redirects []string) error {
	if len(environments) == 0 {
		return nil
	}
	allowed := map[string]string{"DEVELOPMENT": "开发环境", "TEST": "测试环境", "PRODUCTION": "生产环境"}
	redirectSet := make(map[string]struct{}, len(redirects))
	for _, value := range redirects {
		redirectSet[strings.TrimSpace(value)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(environments))
	for _, environment := range environments {
		key := strings.ToUpper(strings.TrimSpace(environment.Key))
		if _, ok := allowed[key]; !ok || strings.TrimSpace(environment.Name) == "" || len(environment.RedirectURIs) == 0 {
			return ErrInvalidIdentityBinding
		}
		if _, exists := seen[key]; exists {
			return ErrInvalidIdentityBinding
		}
		seen[key] = struct{}{}
		for _, redirect := range environment.RedirectURIs {
			normalized := strings.TrimSpace(redirect)
			if _, ok := redirectSet[normalized]; !ok {
				return ErrInvalidIdentityBinding
			}
			parsed, err := url.Parse(normalized)
			if err != nil || (key != "DEVELOPMENT" && !strings.EqualFold(parsed.Scheme, "https")) {
				return ErrInvalidIdentityBinding
			}
		}
	}
	return nil
}

func NormalizeOIDCScopes(values []string) ([]string, error) {
	if len(values) == 0 {
		return append([]string(nil), DefaultOIDCScopes...), nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		if strings.ContainsAny(raw, "\r\n\t,;\"'") {
			return nil, ErrInvalidIdentityBinding
		}
		for _, value := range strings.Fields(raw) {
			value = strings.TrimSpace(value)
			if value == "" || len(value) > 64 {
				return nil, ErrInvalidIdentityBinding
			}
			for _, r := range value {
				if r != '_' && r != '-' && r != '.' && r != ':' && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
					return nil, ErrInvalidIdentityBinding
				}
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
			if len(out) > 32 {
				return nil, ErrInvalidIdentityBinding
			}
		}
	}
	if len(out) == 0 {
		return nil, ErrInvalidIdentityBinding
	}
	return out, nil
}

func validIdentityURL(u *url.URL) bool {
	if u == nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if strings.EqualFold(u.Scheme, "https") {
		return true
	}
	return strings.EqualFold(u.Scheme, "http") && (strings.EqualFold(u.Hostname(), "localhost") || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}

func (b IdentityBinding) RedirectURIsJSON() string {
	data, _ := json.Marshal(b.RedirectURIs)
	return string(data)
}

func DecodeRedirectURIs(raw string) []string {
	var out []string
	if json.Unmarshal([]byte(raw), &out) != nil {
		return nil
	}
	return out
}

func EncodeApplicationEnvironments(environments []ApplicationEnvironment) string {
	names := map[string]string{"DEVELOPMENT": "开发环境", "TEST": "测试环境", "PRODUCTION": "生产环境"}
	normalized := make([]ApplicationEnvironment, 0, len(environments))
	for _, environment := range environments {
		key := strings.ToUpper(strings.TrimSpace(environment.Key))
		redirects := make([]string, 0, len(environment.RedirectURIs))
		for _, redirect := range environment.RedirectURIs {
			redirects = append(redirects, strings.TrimSpace(redirect))
		}
		normalized = append(normalized, ApplicationEnvironment{Key: key, Name: names[key], RedirectURIs: redirects})
	}
	data, _ := json.Marshal(normalized)
	return string(data)
}

func DecodeApplicationEnvironments(raw string) []ApplicationEnvironment {
	var out []ApplicationEnvironment
	if json.Unmarshal([]byte(raw), &out) != nil {
		return nil
	}
	return out
}
