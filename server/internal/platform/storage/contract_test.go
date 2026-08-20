package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appcfg "github.com/sevoniva-labs/velora/server/internal/platform/config"
)

func targetContract(profile ProviderProfile, capabilities ...Capability) CapabilityContract {
	results := map[Capability]CapabilityStatus{}
	for _, capability := range capabilities {
		results[capability] = CapabilityStatus{State: CapabilitySupported, Evidence: "contract test: " + string(capability)}
	}
	return CapabilityContract{
		Profile:        profile,
		Level:          EvidenceTargetTested,
		Target:         "test-target",
		EvidenceRef:    "evidence/storage-contract.json",
		EvidenceDigest: strings.Repeat("a", 64),
		TestedAt:       time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		Capabilities:   results,
	}
}

func TestCapabilityContractRequiresTargetEvidence(t *testing.T) {
	contract := defaultCapabilityContract(ProviderProfileMinIO)
	if err := contract.Validate(CapabilityMultipartRecovery); err == nil {
		t.Fatal("default profile must not enable multipart without target evidence")
	}

	contract = targetContract(ProviderProfileMinIO, CapabilityBasicObjectIO, CapabilityMultipartRecovery)
	if err := contract.Validate(CapabilityMultipartRecovery); err != nil {
		t.Fatalf("target-tested capability contract rejected: %v", err)
	}
}

func TestCapabilityContractRejectsMalformedEvidence(t *testing.T) {
	contract := targetContract(ProviderProfileCephRGW, CapabilityBasicObjectIO)
	contract.EvidenceDigest = "not-a-digest"
	if err := contract.Validate(CapabilityBasicObjectIO); err == nil {
		t.Fatal("malformed evidence digest must fail closed")
	}
}

func TestCapabilityContractRejectsWrongProfile(t *testing.T) {
	contract := targetContract(ProviderProfileAlibabaOSS, CapabilityBasicObjectIO)
	if err := contract.Validate(CapabilityObjectLock); err == nil {
		t.Fatal("unproven capability must fail closed")
	}
}

func writeContractFile(t *testing.T, contract CapabilityContract) string {
	t.Helper()
	contract.EvidenceDigest = ""
	canonical, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(canonical)
	contract.EvidenceDigest = hex.EncodeToString(sum[:])
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "storage-contract.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadCapabilityContractVerifiesDigestAndSchema(t *testing.T) {
	path := writeContractFile(t, targetContract(ProviderProfileMinIO, CapabilityBasicObjectIO))
	got, err := LoadCapabilityContract(path)
	if err != nil {
		t.Fatalf("LoadCapabilityContract() error = %v", err)
	}
	if got.Level != EvidenceTargetTested || got.Capabilities[CapabilityBasicObjectIO].State != CapabilitySupported {
		t.Fatalf("unexpected loaded contract: %#v", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "test-target", "other-target", 1))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCapabilityContract(path); err == nil {
		t.Fatal("tampered contract must fail digest verification")
	}
}

func TestTargetForConfigBindsEndpointRegionBucketAndPrefix(t *testing.T) {
	got := TargetForConfig(appcfg.Storage{Endpoint: "https://s3.example/", Region: "cn", Bucket: "velora", Prefix: "/audit/"})
	if got != "https://s3.example|cn|velora|audit" {
		t.Fatalf("TargetForConfig() = %q", got)
	}
}
