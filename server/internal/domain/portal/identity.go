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
	ConfigurationStatus    string
	VerificationStatus     string
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
	for _, raw := range b.RedirectURIs {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || !validIdentityURL(u) {
			return ErrInvalidIdentityBinding
		}
	}
	return nil
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
