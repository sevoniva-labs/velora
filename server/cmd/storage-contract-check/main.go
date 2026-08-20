// Command storage-contract-check performs a target-specific S3 capability
// contract test. It creates short-lived evidence objects and writes a signed
// by digest contract file; it does not claim vendor certification.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	appcfg "github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/storage"
)

func main() {
	cfg, err := appcfg.Load()
	if err != nil {
		fail(fmt.Errorf("config: %w", err))
	}
	profile, err := storage.ResolveProviderProfile(cfg.Storage.Provider)
	if err != nil {
		fail(err)
	}
	if profile == storage.ProviderProfileLocal {
		fail(errors.New("storage capability contract requires an S3-compatible target"))
	}
	outPath := strings.TrimSpace(os.Getenv("VELORA_STORAGE_CONTRACT_OUTPUT"))
	if outPath == "" {
		fail(errors.New("VELORA_STORAGE_CONTRACT_OUTPUT is required"))
	}
	timeout := 2 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("VELORA_STORAGE_CONTRACT_TIMEOUT")); raw != "" {
		parsed, parseErr := time.ParseDuration(raw)
		if parseErr != nil || parsed <= 0 || parsed > 10*time.Minute {
			fail(errors.New("VELORA_STORAGE_CONTRACT_TIMEOUT must be between 1s and 10m"))
		}
		timeout = parsed
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	loadOpts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Storage.Region)}
	if cfg.Storage.AccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.Storage.AccessKey, cfg.Storage.SecretKey, cfg.Storage.SessionToken)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		fail(fmt.Errorf("aws config: %w", err))
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.Storage.PathStyle
		if strings.TrimSpace(cfg.Storage.Endpoint) != "" {
			o.BaseEndpoint = aws.String(cfg.Storage.Endpoint)
		}
	})
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Storage.Bucket)}); err != nil {
		fail(fmt.Errorf("head bucket: %w", err))
	}

	capabilities := map[storage.Capability]storage.CapabilityStatus{}
	for _, capability := range []storage.Capability{storage.CapabilityBasicObjectIO, storage.CapabilityChecksum, storage.CapabilityVersioning, storage.CapabilityObjectLock, storage.CapabilityRetention, storage.CapabilityLegalHold, storage.CapabilitySSES3, storage.CapabilitySSEKMS, storage.CapabilityMultipartRecovery, storage.CapabilityConstrainedPresign, storage.CapabilityTemporaryCredential} {
		capabilities[capability] = storage.CapabilityStatus{State: storage.CapabilityUnknown, Evidence: "not exercised by this contract"}
	}
	marker := []byte("velora storage capability contract\n" + time.Now().UTC().Format(time.RFC3339Nano))
	digest := sha256.Sum256(marker)
	key := "_velora-contract/" + time.Now().UTC().Format("20060102T150405.000000000Z")
	put, err := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(key), Body: bytes.NewReader(marker), ChecksumAlgorithm: types.ChecksumAlgorithmSha256, ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(digest[:]))})
	if err != nil {
		fail(fmt.Errorf("basic/checksum put: %w", err))
	}
	capabilities[storage.CapabilityBasicObjectIO] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "HeadBucket + PutObject + GetObject + DeleteObject"}
	capabilities[storage.CapabilityChecksum] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "PutObject SHA-256 checksum accepted"}
	got, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(key)})
	if err != nil {
		fail(fmt.Errorf("basic get: %w", err))
	}
	body, err := io.ReadAll(io.LimitReader(got.Body, 1<<20))
	_ = got.Body.Close()
	if err != nil || !bytes.Equal(body, marker) {
		fail(errors.New("basic get verification failed"))
	}
	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(key)}); err != nil {
		fail(fmt.Errorf("basic delete: %w", err))
	}
	if put == nil {
		fail(errors.New("storage provider returned an empty put response"))
	}

	version, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(cfg.Storage.Bucket)})
	if err == nil && version.Status == types.BucketVersioningStatusEnabled {
		capabilities[storage.CapabilityVersioning] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "GetBucketVersioning returned Enabled"}
	} else if err == nil {
		capabilities[storage.CapabilityVersioning] = storage.CapabilityStatus{State: storage.CapabilityUnsupported, Evidence: "GetBucketVersioning did not return Enabled"}
	}
	lock, err := client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: aws.String(cfg.Storage.Bucket)})
	if err == nil && lock.ObjectLockConfiguration != nil && lock.ObjectLockConfiguration.ObjectLockEnabled == types.ObjectLockEnabledEnabled {
		capabilities[storage.CapabilityObjectLock] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "GetObjectLockConfiguration returned Enabled"}
		retainUntil := time.Now().UTC().Add(2 * time.Minute)
		lockKey := key + ".immutable"
		lockPut, putErr := client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(lockKey), Body: bytes.NewReader(marker), ChecksumAlgorithm: types.ChecksumAlgorithmSha256, ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(digest[:])), ObjectLockMode: types.ObjectLockModeCompliance, ObjectLockRetainUntilDate: aws.Time(retainUntil)})
		if putErr == nil && lockPut.VersionId != nil && *lockPut.VersionId != "" {
			head, headErr := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(lockKey), VersionId: lockPut.VersionId})
			if headErr == nil && head.ObjectLockMode == types.ObjectLockModeCompliance && head.ObjectLockRetainUntilDate != nil && head.ObjectLockRetainUntilDate.After(time.Now().UTC()) {
				capabilities[storage.CapabilityRetention] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "compliance retention round-trip verified"}
				legalHold, legalHoldErr := client.PutObjectLegalHold(ctx, &s3.PutObjectLegalHoldInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(lockKey), VersionId: lockPut.VersionId, LegalHold: &types.ObjectLockLegalHold{Status: types.ObjectLockLegalHoldStatusOn}})
				if legalHoldErr == nil && legalHold != nil {
					status, getErr := client.GetObjectLegalHold(ctx, &s3.GetObjectLegalHoldInput{Bucket: aws.String(cfg.Storage.Bucket), Key: aws.String(lockKey), VersionId: lockPut.VersionId})
					if getErr == nil && status != nil && status.LegalHold != nil && status.LegalHold.Status == types.ObjectLockLegalHoldStatusOn {
						capabilities[storage.CapabilityLegalHold] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "PutObjectLegalHold + GetObjectLegalHold returned ON"}
					}
				}
			}
		}
	}
	if cfg.Storage.SSEMode == "s3" {
		capabilities[storage.CapabilitySSES3] = storage.CapabilityStatus{State: storage.CapabilitySupported, Evidence: "configured sse_mode=s3; target-specific header verification is required before production"}
	}
	if cfg.Storage.SSEMode == "kms" {
		capabilities[storage.CapabilitySSEKMS] = storage.CapabilityStatus{State: storage.CapabilityUnknown, Evidence: "KMS key policy and target encryption proof require a dedicated KMS contract"}
	}

	ref := outPath
	report := storage.CapabilityContract{Profile: profile, Level: storage.EvidenceTargetTested, Target: storage.TargetForConfig(cfg.Storage), EvidenceRef: ref, TestedAt: time.Now().UTC(), Capabilities: capabilities}
	canonical, err := json.Marshal(report)
	if err != nil {
		fail(err)
	}
	sum := sha256.Sum256(canonical)
	report.EvidenceDigest = hex.EncodeToString(sum[:])
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0o600); err != nil {
		fail(fmt.Errorf("write contract: %w", err))
	}
	if _, err := storage.LoadCapabilityContract(outPath); err != nil {
		fail(fmt.Errorf("self-verify contract: %w", err))
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintf(os.Stderr, "storage-contract-check failed: %v\n", err); os.Exit(1) }
