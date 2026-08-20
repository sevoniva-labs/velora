package crypto

import (
	"context"
	"errors"
	"strings"
)

// KeySource is the adapter slot for an organization-approved KMS, HSM, or
// password-device product. Raw key material must not cross this boundary in a
// production adapter.
type KeySource interface {
	Activate(ctx context.Context, keyID, version string) error
}

// DualControl is implemented by the approval/audit integration that enforces
// two distinct operators and the institution's dual-control policy.
type DualControl interface {
	Authorize(ctx context.Context, approvalID, keyID, version, firstOperator, secondOperator string) error
}

type RotationRequest struct {
	ApprovalID     string
	KeyID          string
	NewVersion     string
	FirstOperator  string
	SecondOperator string
}

type KeyManager struct {
	source  KeySource
	control DualControl
}

func NewKeyManager(source KeySource, control DualControl) (*KeyManager, error) {
	if source == nil || control == nil {
		return nil, errors.New("key source and dual-control adapter are required")
	}
	return &KeyManager{source: source, control: control}, nil
}

func (m *KeyManager) Rotate(ctx context.Context, request RotationRequest) error {
	request.ApprovalID = strings.TrimSpace(request.ApprovalID)
	request.KeyID = strings.TrimSpace(request.KeyID)
	request.NewVersion = strings.TrimSpace(request.NewVersion)
	request.FirstOperator = strings.TrimSpace(request.FirstOperator)
	request.SecondOperator = strings.TrimSpace(request.SecondOperator)
	if request.ApprovalID == "" || request.KeyID == "" || request.NewVersion == "" || request.FirstOperator == "" || request.SecondOperator == "" || request.FirstOperator == request.SecondOperator || strings.ContainsAny(request.NewVersion, ".\r\n\t ") {
		return errors.New("key rotation requires approval, key, version, and two distinct operators")
	}
	if err := m.control.Authorize(ctx, request.ApprovalID, request.KeyID, request.NewVersion, request.FirstOperator, request.SecondOperator); err != nil {
		return err
	}
	return m.source.Activate(ctx, request.KeyID, request.NewVersion)
}
