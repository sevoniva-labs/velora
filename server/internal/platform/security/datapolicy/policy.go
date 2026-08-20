package datapolicy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/security/masking"
)

type Classification string

const (
	ClassificationPublic              Classification = "public"
	ClassificationInternal            Classification = "internal"
	ClassificationConfidential        Classification = "confidential"
	ClassificationRestricted          Classification = "restricted"
	ClassificationPersonalInformation Classification = "personal_information"
	ClassificationImportantData       Classification = "important_data"
)

type MaskStrategy string

const (
	MaskNone     MaskStrategy = "none"
	MaskStars    MaskStrategy = "stars"
	MaskMobile   MaskStrategy = "mobile"
	MaskIDCard   MaskStrategy = "id_card"
	MaskBankCard MaskStrategy = "bank_card"
	MaskEmail    MaskStrategy = "email"
	MaskName     MaskStrategy = "name"
)

type FieldPolicy struct {
	Key            string
	Classification Classification
	Owner          string
	Purpose        string
	Residency      string
	RetentionDays  int
	Tags           []string
	Mask           MaskStrategy
	ExportApproval bool
	Watermark      bool
}

// Record adds tenant and lifecycle metadata to a field policy persisted by the
// platform. The policy itself remains independent from transport and storage.
type Record struct {
	ID             string
	OrganizationID string
	FieldPolicy
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DeletionEvidence is an append-only proof record. ResourceDigest is a
// caller-provided digest so the platform never needs to persist raw business
// identifiers in the governance log.
type DeletionEvidence struct {
	ID             string
	OrganizationID string
	ResourceType   string
	ResourceDigest string
	FieldKeys      []string
	Reason         string
	RecordsDeleted int64
	DeletedAt      time.Time
	EvidenceHash   string
	CreatedAt      time.Time
}

type ExportRequest struct {
	ApprovalID string
	Purpose    string
	Watermark  string
}

type Catalog struct {
	fields map[string]FieldPolicy
}

func NewCatalog(policies []FieldPolicy) (*Catalog, error) {
	fields := make(map[string]FieldPolicy, len(policies))
	for _, policy := range policies {
		policy.Key = strings.TrimSpace(policy.Key)
		policy.Owner = strings.TrimSpace(policy.Owner)
		policy.Purpose = strings.TrimSpace(policy.Purpose)
		policy.Residency = strings.TrimSpace(policy.Residency)
		if policy.Key == "" || policy.Owner == "" || policy.Purpose == "" || policy.Residency == "" {
			return nil, errors.New("data field policy requires key, owner, purpose and residency")
		}
		if _, exists := fields[policy.Key]; exists {
			return nil, fmt.Errorf("duplicate data field policy %q", policy.Key)
		}
		if policy.RetentionDays <= 0 {
			return nil, fmt.Errorf("data field policy %q requires positive retention days", policy.Key)
		}
		if !validClassification(policy.Classification) {
			return nil, fmt.Errorf("data field policy %q has unknown classification", policy.Key)
		}
		if policy.Classification == ClassificationPersonalInformation || policy.Classification == ClassificationRestricted {
			if policy.Mask == MaskNone {
				return nil, fmt.Errorf("data field policy %q requires masking", policy.Key)
			}
			policy.ExportApproval = true
			policy.Watermark = true
		}
		if !validMask(policy.Mask) {
			return nil, fmt.Errorf("data field policy %q has unknown mask strategy", policy.Key)
		}
		policy.Tags = append([]string(nil), policy.Tags...)
		fields[policy.Key] = policy
	}
	return &Catalog{fields: fields}, nil
}

func (c *Catalog) Policy(key string) (FieldPolicy, error) {
	policy, ok := c.fields[strings.TrimSpace(key)]
	if !ok {
		return FieldPolicy{}, fmt.Errorf("data field policy %q is not registered", key)
	}
	policy.Tags = append([]string(nil), policy.Tags...)
	return policy, nil
}

func (c *Catalog) MaskValue(key, value string) (string, error) {
	policy, err := c.Policy(key)
	if err != nil {
		return "", err
	}
	switch policy.Mask {
	case MaskNone:
		return value, nil
	case MaskStars:
		return masking.Stars(value), nil
	case MaskMobile:
		return masking.Mobile(value), nil
	case MaskIDCard:
		return masking.IDCard(value), nil
	case MaskBankCard:
		return masking.BankCard(value), nil
	case MaskEmail:
		return masking.Email(value), nil
	case MaskName:
		return masking.Name(value), nil
	default:
		return "", fmt.Errorf("data field policy %q has unsupported mask strategy", key)
	}
}

func (c *Catalog) AuthorizeExport(keys []string, request ExportRequest) error {
	if strings.TrimSpace(request.Purpose) == "" {
		return errors.New("data export purpose is required")
	}
	for _, key := range keys {
		policy, err := c.Policy(key)
		if err != nil {
			return err
		}
		if policy.ExportApproval && strings.TrimSpace(request.ApprovalID) == "" {
			return fmt.Errorf("data export for %q requires approval", key)
		}
		if policy.Watermark && strings.TrimSpace(request.Watermark) == "" {
			return fmt.Errorf("data export for %q requires watermark", key)
		}
	}
	return nil
}

func validClassification(value Classification) bool {
	switch value {
	case ClassificationPublic, ClassificationInternal, ClassificationConfidential, ClassificationRestricted, ClassificationPersonalInformation, ClassificationImportantData:
		return true
	default:
		return false
	}
}

func validMask(value MaskStrategy) bool {
	switch value {
	case MaskNone, MaskStars, MaskMobile, MaskIDCard, MaskBankCard, MaskEmail, MaskName:
		return true
	default:
		return false
	}
}
