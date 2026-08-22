// Command crypto-rotation-check exercises the local crypto governance contract.
// It deliberately uses in-memory adapters: production key material must remain
// inside an approved KMS/HSM/PKCS#11 adapter, and an uninstalled adapter fails
// closed at bootstrap.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appcrypto "github.com/sevoniva-labs/velora/server/internal/platform/security/crypto"
)

type fakeSource struct {
	activations []string
}

func (s *fakeSource) Activate(_ context.Context, keyID, version string) error {
	s.activations = append(s.activations, keyID+"/"+version)
	return nil
}

type fakeControl struct{ err error }

func (c *fakeControl) Authorize(_ context.Context, _, _, _, _, _ string) error { return c.err }

type evidence struct {
	Status                     string    `json:"status"`
	CheckedAt                  time.Time `json:"checked_at"`
	TwoPersonApproval          string    `json:"two_person_approval"`
	ApprovalDenyFailsClosed    bool      `json:"approval_deny_fails_closed"`
	SoftwareGMRoundTrip        bool      `json:"software_gm_round_trip"`
	HardwareAdapterFailsClosed bool      `json:"hardware_adapter_fails_closed"`
	Certification              string    `json:"certification"`
}

func main() {
	ctx := context.Background()
	source := &fakeSource{}
	control := &fakeControl{}
	manager, err := appcrypto.NewKeyManager(source, control)
	if err != nil {
		fail(err)
	}
	invalid := appcrypto.RotationRequest{ApprovalID: "approval-local", KeyID: "audit-kek", NewVersion: "v2", FirstOperator: "alice", SecondOperator: "alice"}
	if err := manager.Rotate(ctx, invalid); err == nil {
		fail(errors.New("same operator was accepted by dual control"))
	}
	if len(source.activations) != 0 {
		fail(errors.New("invalid dual-control request activated a key"))
	}
	control.err = errors.New("approval denied")
	if err := manager.Rotate(ctx, appcrypto.RotationRequest{ApprovalID: "approval-local", KeyID: "audit-kek", NewVersion: "v2", FirstOperator: "alice", SecondOperator: "bob"}); err == nil {
		fail(errors.New("denied key rotation succeeded"))
	}
	if len(source.activations) != 0 {
		fail(errors.New("denied key rotation activated a key"))
	}
	control.err = nil
	if err := manager.Rotate(ctx, appcrypto.RotationRequest{ApprovalID: "approval-local", KeyID: "audit-kek", NewVersion: "v2", FirstOperator: "alice", SecondOperator: "bob"}); err != nil {
		fail(err)
	}
	if len(source.activations) != 1 || source.activations[0] != "audit-kek/v2" {
		fail(fmt.Errorf("unexpected activation evidence: %v", source.activations))
	}

	gm, err := appcrypto.New("gm", "local-contract-key-material-0123456789abcdef")
	if err != nil {
		fail(err)
	}
	ciphertext, err := gm.Encrypt([]byte("local crypto contract"), []byte("acceptance"))
	if err != nil {
		fail(err)
	}
	plaintext, err := gm.Decrypt(ciphertext, []byte("acceptance"))
	if err != nil || string(plaintext) != "local crypto contract" {
		fail(fmt.Errorf("software GM round trip failed: %w", err))
	}
	if _, err := appcrypto.NewWithAdapter("gm", "hsm", "local-contract-key-material-0123456789abcdef"); err == nil {
		fail(errors.New("uninstalled HSM adapter did not fail closed"))
	}

	report := evidence{
		Status:                     "passed",
		CheckedAt:                  time.Now().UTC(),
		TwoPersonApproval:          "approved distinct operators activated one version; same operator rejected",
		ApprovalDenyFailsClosed:    true,
		SoftwareGMRoundTrip:        true,
		HardwareAdapterFailsClosed: true,
		Certification:              "not_certified: software GM is a local algorithm contract only; approved KMS/HSM/国密 adapter and certification remain required",
	}
	if path := strings.TrimSpace(os.Getenv("VELORA_ACCEPTANCE_EVIDENCE_DIR")); path != "" {
		// #nosec G703 -- this acceptance-only command writes to an operator-selected evidence directory, never a request-derived path.
		if err := os.MkdirAll(path, 0o750); err != nil {
			fail(fmt.Errorf("create evidence directory: %w", err))
		}
		name := filepath.Join(path, "crypto-rotation-"+time.Now().UTC().Format("20060102T150405Z")+".json")
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fail(err)
		}
		// #nosec G703 -- name is a fixed timestamped filename beneath the operator-selected evidence directory.
		if err := os.WriteFile(name, append(data, '\n'), 0o600); err != nil {
			fail(fmt.Errorf("write evidence: %w", err))
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "crypto-rotation-check failed: %v\n", err)
	os.Exit(1)
}
