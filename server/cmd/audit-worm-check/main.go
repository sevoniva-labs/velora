// Command audit-worm-check archives a representative audit batch to an
// object-lock target, verifies the receipt, and proves that the retained
// version cannot be deleted. It is an acceptance test, not a vendor or
// compliance certification.
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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sevoniva-labs/velora/server/internal/adapters/auditarchive"
	"github.com/sevoniva-labs/velora/server/internal/adapters/auditsink"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	appcfg "github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/storage"
)

type evidence struct {
	Status                   string    `json:"status"`
	CheckedAt                time.Time `json:"checked_at"`
	Provider                 string    `json:"provider"`
	EventCount               int       `json:"event_count"`
	ReceiptVerified          bool      `json:"receipt_verified"`
	RetainedVersionDeleteRef string    `json:"retained_version_delete_refusal"`
	SIEMForwarder            string    `json:"siem_forwarder"`
	Certification            string    `json:"certification"`
}

func main() {
	cfg, err := appcfg.Load()
	if err != nil {
		fail(fmt.Errorf("config: %w", err))
	}
	if _, err := storage.ResolveProviderProfile(cfg.Storage.Provider); err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		fail(fmt.Errorf("storage: %w", err))
	}
	archive, err := auditarchive.NewS3Archive(store, "local-acceptance")
	if err != nil {
		fail(fmt.Errorf("worm archive adapter: %w", err))
	}
	batch := audit.ArchiveBatch{Events: []audit.Event{
		{
			ID: "acceptance-audit-1", OccurredAt: time.Now().UTC(), RequestID: "acceptance-request-1",
			OrganizationID: "acceptance-org", ActorID: "acceptance-operator", Action: "acceptance.worm.verify",
			ResourceType: "acceptance", ResourceID: "worm-contract", Result: "SUCCESS",
			Details: map[string]any{"purpose": "local production acceptance"},
		},
	}}
	retainUntil := time.Now().UTC().Add(5 * time.Minute)
	receipt, err := archive.Archive(ctx, batch, retainUntil)
	if err != nil {
		fail(fmt.Errorf("archive: %w", err))
	}
	if err := audit.ValidateArchiveReceipt(batch, receipt, time.Now().UTC()); err != nil {
		fail(fmt.Errorf("receipt: %w", err))
	}
	if err := archive.Verify(ctx, receipt); err != nil {
		fail(fmt.Errorf("immutable verification: %w", err))
	}

	client, err := newS3Client(ctx, cfg.Storage)
	if err != nil {
		fail(err)
	}
	physicalKey := strings.Trim(strings.TrimSpace(cfg.Storage.Prefix), "/")
	if physicalKey != "" {
		physicalKey += "/"
	}
	physicalKey += receipt.ObjectKey
	_, deleteErr := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(physicalKey), VersionId: aws.String(receipt.VersionID),
	})
	if deleteErr == nil {
		fail(errors.New("object-lock compliance allowed deleting the retained version"))
	}
	if err := archive.Verify(ctx, receipt); err != nil {
		fail(fmt.Errorf("immutable object changed after rejected delete: %w", err))
	}

	forwarder, err := auditsink.NewReliableForwarder("audit-events")
	if err != nil {
		fail(fmt.Errorf("SIEM forwarder: %w", err))
	}
	report := evidence{
		Status:                   "passed",
		CheckedAt:                time.Now().UTC(),
		Provider:                 archive.Provider(),
		EventCount:               len(batch.Events),
		ReceiptVerified:          true,
		RetainedVersionDeleteRef: "version-specific delete rejected and receipt remained verifiable",
		SIEMForwarder:            forwarder.Provider() + "; durable enqueue contract present, external SIEM delivery still requires deployment validation",
		Certification:            "not_certified: local target evidence only; production WORM policy, SIEM endpoint, retention/legal approval and recovery drills remain required",
	}
	if path := strings.TrimSpace(os.Getenv("VELORA_ACCEPTANCE_EVIDENCE_DIR")); path != "" {
		if err := os.MkdirAll(path, 0o750); err != nil {
			fail(fmt.Errorf("create evidence directory: %w", err))
		}
		name := filepath.Join(path, "audit-worm-"+time.Now().UTC().Format("20060102T150405Z")+".json")
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fail(err)
		}
		if err := os.WriteFile(name, append(data, '\n'), 0o600); err != nil {
			fail(fmt.Errorf("write evidence: %w", err))
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err)
	}
}

func newS3Client(ctx context.Context, c appcfg.Storage) (*s3.Client, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(c.Region)}
	if c.AccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.SessionToken)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = c.PathStyle
		if strings.TrimSpace(c.Endpoint) != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
	}), nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "audit-worm-check failed: %v\n", err)
	os.Exit(1)
}
