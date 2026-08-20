package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
)

func TestLocalStoreRejectsTraversalAndEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	store, err := New(context.Background(), config.Storage{Provider: "local", LocalRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../outside", "/absolute", "escape/secret"} {
		if err := store.Put(context.Background(), key, strings.NewReader("secret")); err == nil {
			t.Fatalf("unsafe key %q was accepted", key)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "secret")); !os.IsNotExist(err) {
		t.Fatalf("write escaped local root: %v", err)
	}
}

func TestLocalStoreRoundTripUsesPrivateFileMode(t *testing.T) {
	root := t.TempDir()
	store, err := New(context.Background(), config.Storage{Provider: "local", LocalRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Put(ctx, "documents/report.txt", strings.NewReader("banking")); err != nil {
		t.Fatal(err)
	}
	r, err := store.Get(ctx, "documents/report.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	got, err := io.ReadAll(r)
	if err != nil || string(got) != "banking" {
		t.Fatalf("round trip got %q, err %v", got, err)
	}
	info, err := os.Stat(filepath.Join(root, "documents", "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestObjectStoreContractNormalizesAndRejectsUnsafeKeys(t *testing.T) {
	root := t.TempDir()
	legacy, err := New(context.Background(), config.Storage{Provider: "local", LocalRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewObjectStore(legacy)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.Put(ctx, "tenant/a.txt", strings.NewReader("ok"), 2, PutOptions{ContentType: "text/plain"}); err != nil {
		t.Fatal(err)
	}
	body, info, err := store.Get(ctx, "tenant/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = body.Close() }()
	if info.Size != 2 {
		t.Fatalf("object size = %d, want 2", info.Size)
	}
	for _, key := range []string{"../escape", "a/../b", `a\\b`, "/absolute"} {
		if _, err := store.Put(ctx, key, strings.NewReader("x"), 1, PutOptions{}); err == nil {
			t.Fatalf("unsafe object key %q was accepted", key)
		}
	}
}

func TestStorageCapabilitiesFailClosedUntilContractEvidenceExists(t *testing.T) {
	store, err := New(context.Background(), config.Storage{Provider: "s3", Endpoint: "https://s3.example", Region: "cn-beijing", Bucket: "documents", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireCapabilities(store, CapabilityBasicObjectIO); err != nil {
		t.Fatalf("basic S3 capability rejected: %v", err)
	}
	if err := RequireCapabilities(store, CapabilityObjectLock); err == nil {
		t.Fatal("unverified object lock capability was accepted")
	}
}

func TestLocalStorageDoesNotClaimS3Capabilities(t *testing.T) {
	store, err := New(context.Background(), config.Storage{Provider: "local", LocalRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireCapabilities(store, CapabilityObjectLock); err == nil {
		t.Fatal("local provider claimed object lock")
	}
}

func TestResolveProviderProfileKeepsVendorEvidenceBoundary(t *testing.T) {
	tests := map[string]ProviderProfile{
		"s3":            ProviderProfileAWSS3,
		"minio":         ProviderProfileMinIO,
		"ceph-rgw":      ProviderProfileCephRGW,
		"oss":           ProviderProfileAlibabaOSS,
		"cos":           ProviderProfileTencentCOS,
		"obs":           ProviderProfileHuaweiOBS,
		"s3-compatible": ProviderProfileGenericS3,
	}
	for input, want := range tests {
		got, err := ResolveProviderProfile(input)
		if err != nil || got != want {
			t.Fatalf("ResolveProviderProfile(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ResolveProviderProfile("unknown-object-store"); err == nil {
		t.Fatal("unknown provider profile was accepted")
	}
}

func TestS3StoreReportsExplicitProfile(t *testing.T) {
	store, err := New(context.Background(), config.Storage{Provider: "cos", Endpoint: "https://cos.example", Region: "ap-shanghai", Bucket: "documents", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	reporter, ok := store.(ProfileReporter)
	if !ok {
		t.Fatal("storage store does not report a provider profile")
	}
	if reporter.Profile() != ProviderProfileTencentCOS {
		t.Fatalf("storage profile = %v, want %q", reporter.Profile(), ProviderProfileTencentCOS)
	}
}

func TestS3ImmutableStoreFailsClosedWithoutTargetEvidence(t *testing.T) {
	store, err := New(context.Background(), config.Storage{Provider: "s3", Endpoint: "https://s3.example", Region: "cn-beijing", Bucket: "audit", TLS: true})
	if err != nil {
		t.Fatal(err)
	}
	immutable, ok := store.(ImmutableStore)
	if !ok {
		t.Fatal("s3 store does not expose immutable contract")
	}
	if _, err := immutable.PutImmutable(context.Background(), "audit/event.json", []byte(`{"event":1}`), time.Now().UTC().Add(time.Hour)); err == nil {
		t.Fatal("immutable put was accepted without target capability evidence")
	}
}
