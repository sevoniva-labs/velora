package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// EvidenceLevel is the governance label attached to a provider capability.
// A profile is not a certification and must not enable advanced operations.
type EvidenceLevel string

const (
	EvidenceBuiltIn      EvidenceLevel = "Built-in"
	EvidenceProfile      EvidenceLevel = "Profile"
	EvidenceAdapterSlot  EvidenceLevel = "Adapter slot"
	EvidenceExperimental EvidenceLevel = "Experimental"
	EvidenceTargetTested EvidenceLevel = "Target-tested"
	EvidenceNotCertified EvidenceLevel = "Not certified"
)

// CapabilityContract is evidence produced by a target-specific contract test.
// EvidenceDigest is the SHA-256 digest of the immutable evidence artifact.
type CapabilityContract struct {
	Profile        ProviderProfile                 `json:"profile"`
	Level          EvidenceLevel                   `json:"level"`
	Target         string                          `json:"target"`
	EvidenceRef    string                          `json:"evidence_ref"`
	EvidenceDigest string                          `json:"evidence_digest"`
	TestedAt       time.Time                       `json:"tested_at"`
	Capabilities   map[Capability]CapabilityStatus `json:"capabilities"`
}

func (c CapabilityContract) Validate(required ...Capability) error {
	if !IsS3Profile(c.Profile) {
		return errors.New("capability contract requires an s3 provider profile")
	}
	if c.Level != EvidenceTargetTested {
		return fmt.Errorf("storage capability contract level %q is not target-tested", c.Level)
	}
	if strings.TrimSpace(c.Target) == "" || strings.TrimSpace(c.EvidenceRef) == "" || c.TestedAt.IsZero() {
		return errors.New("storage capability contract target, evidence reference, and test time are required")
	}
	digest := strings.TrimSpace(c.EvidenceDigest)
	if len(digest) != 64 {
		return errors.New("storage capability contract requires a sha256 evidence digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return errors.New("storage capability contract evidence digest is not hexadecimal")
	}
	if len(c.Capabilities) == 0 {
		return errors.New("storage capability contract has no capability results")
	}
	for capability, status := range c.Capabilities {
		if status.State == CapabilitySupported && strings.TrimSpace(status.Evidence) == "" {
			return fmt.Errorf("supported storage capability %q has no evidence", capability)
		}
	}
	for _, capability := range required {
		status, ok := c.Capabilities[capability]
		if !ok || status.State != CapabilitySupported {
			return fmt.Errorf("storage capability %q is not target-tested and supported", capability)
		}
	}
	return nil
}

// CapabilityContractReporter exposes the immutable target evidence used by a store.
type CapabilityContractReporter interface {
	CapabilityContract() CapabilityContract
}

// RequireTargetTestedCapabilities fails closed unless the target evidence explicitly
// proves every requested capability. Provider product names never bypass this check.
func RequireTargetTestedCapabilities(store Store, required ...Capability) error {
	reporter, ok := store.(CapabilityContractReporter)
	if !ok {
		return errors.New("storage provider does not report a capability contract")
	}
	if err := reporter.CapabilityContract().Validate(required...); err != nil {
		return err
	}
	return nil
}

// LoadCapabilityContract reads and verifies a target-specific contract file.
// The evidence digest covers the canonical contract content with the digest
// field blank, preventing an edited report from silently enabling advanced
// object operations.
func LoadCapabilityContract(path string) (CapabilityContract, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return CapabilityContract{}, errors.New("storage capability contract file is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return CapabilityContract{}, fmt.Errorf("read storage capability contract: %w", err)
	}
	var contract CapabilityContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		return CapabilityContract{}, fmt.Errorf("parse storage capability contract: %w", err)
	}
	want := strings.TrimSpace(contract.EvidenceDigest)
	contract.EvidenceDigest = ""
	canonical, err := json.Marshal(contract)
	if err != nil {
		return CapabilityContract{}, fmt.Errorf("canonicalize storage capability contract: %w", err)
	}
	digest := sha256.Sum256(canonical)
	if !strings.EqualFold(want, hex.EncodeToString(digest[:])) {
		return CapabilityContract{}, errors.New("storage capability contract evidence digest does not match file content")
	}
	contract.EvidenceDigest = want
	if err := contract.Validate(); err != nil {
		return CapabilityContract{}, fmt.Errorf("validate storage capability contract: %w", err)
	}
	return contract, nil
}

func defaultCapabilityContract(profile ProviderProfile) CapabilityContract {
	return CapabilityContract{
		Profile: profile,
		Level:   EvidenceNotCertified,
		Capabilities: map[Capability]CapabilityStatus{
			CapabilityBasicObjectIO:       {State: CapabilitySupported, Evidence: "S3 API profile; target contract test required for certification"},
			CapabilityMultipartRecovery:   {State: CapabilityUnknown, Evidence: "target contract test required"},
			CapabilityChecksum:            {State: CapabilityUnknown, Evidence: "target contract test required"},
			CapabilitySSES3:               {State: CapabilityUnknown, Evidence: "target contract test required"},
			CapabilitySSEKMS:              {State: CapabilityUnknown, Evidence: "target KMS contract test required"},
			CapabilityVersioning:          {State: CapabilityUnknown, Evidence: "target contract test required"},
			CapabilityObjectLock:          {State: CapabilityUnknown, Evidence: "target object-lock contract test required"},
			CapabilityRetention:           {State: CapabilityUnknown, Evidence: "target retention contract test required"},
			CapabilityLegalHold:           {State: CapabilityUnknown, Evidence: "target legal-hold contract test required"},
			CapabilityConstrainedPresign:  {State: CapabilityUnknown, Evidence: "target presign contract test required"},
			CapabilityTemporaryCredential: {State: CapabilityUnknown, Evidence: "target STS contract test required"},
		},
	}
}
