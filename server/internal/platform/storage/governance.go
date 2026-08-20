package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type ObjectWriteOptions struct {
	ContentType    string
	ChecksumSHA256 string
	SSES3          bool
	SSEKMSKeyID    string
	RetainUntil    time.Time
	LegalHold      bool
}

type ObjectReceipt struct {
	Key            string
	VersionID      string
	ETag           string
	ChecksumSHA256 string
}

// GovernanceStore is the common S3 protocol surface for regulated objects.
// A provider must pass target capability evidence before any advanced method
// sends a request; product names do not enable these methods.
type GovernanceStore interface {
	Store
	PutGoverned(context.Context, string, []byte, ObjectWriteOptions) (ObjectReceipt, error)
	GetVersion(context.Context, string, string) (io.ReadCloser, error)
	VersioningStatus(context.Context) (string, error)
	EnableVersioning(context.Context) error
	SetRetention(context.Context, string, string, types.ObjectLockRetentionMode, time.Time) error
	SetLegalHold(context.Context, string, string, bool) error
}

type TemporaryCredential struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ExpiresAt       time.Time
}

// TemporaryCredentialIssuer is an adapter slot because S3-compatible targets
// expose different STS endpoints and credential policies.
type TemporaryCredentialIssuer interface {
	IssueTemporaryCredentials(context.Context, time.Duration) (TemporaryCredential, error)
}

func validateGovernedWrite(key string, options ObjectWriteOptions) error {
	if err := validateMultipartKey(key); err != nil {
		return err
	}
	if options.SSES3 && strings.TrimSpace(options.SSEKMSKeyID) != "" {
		return errors.New("sse-s3 and sse-kms are mutually exclusive")
	}
	if (options.LegalHold || !options.RetainUntil.IsZero()) && strings.TrimSpace(options.ChecksumSHA256) == "" {
		return errors.New("object retention and legal hold require an object checksum")
	}
	if !options.RetainUntil.IsZero() && !options.RetainUntil.After(time.Now().UTC()) {
		return errors.New("object retention must be in the future")
	}
	if options.ChecksumSHA256 != "" {
		decoded, err := base64.StdEncoding.DecodeString(options.ChecksumSHA256)
		if err != nil || len(decoded) != 32 {
			return errors.New("checksum sha256 must be a base64 encoded 32-byte digest")
		}
	}
	return nil
}

func (s *s3Store) PutGoverned(ctx context.Context, key string, payload []byte, options ObjectWriteOptions) (ObjectReceipt, error) {
	if err := validateGovernedWrite(key, options); err != nil {
		return ObjectReceipt{}, err
	}
	required := []Capability{CapabilityBasicObjectIO}
	if options.ChecksumSHA256 != "" {
		required = append(required, CapabilityChecksum)
	}
	if options.SSES3 {
		required = append(required, CapabilitySSES3)
	}
	if strings.TrimSpace(options.SSEKMSKeyID) != "" {
		required = append(required, CapabilitySSEKMS)
	}
	if !options.RetainUntil.IsZero() {
		required = append(required, CapabilityVersioning, CapabilityObjectLock, CapabilityRetention, CapabilityChecksum)
	}
	if options.LegalHold {
		required = append(required, CapabilityVersioning, CapabilityObjectLock, CapabilityLegalHold, CapabilityChecksum)
	}
	if err := RequireTargetTestedCapabilities(s, required...); err != nil {
		return ObjectReceipt{}, err
	}
	input := &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: bytes.NewReader(payload)}
	if options.ContentType != "" {
		input.ContentType = aws.String(options.ContentType)
	}
	if options.ChecksumSHA256 != "" {
		input.ChecksumAlgorithm = types.ChecksumAlgorithmSha256
		input.ChecksumSHA256 = aws.String(options.ChecksumSHA256)
	}
	if options.SSES3 {
		input.ServerSideEncryption = types.ServerSideEncryptionAes256
	}
	if options.SSEKMSKeyID != "" {
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(options.SSEKMSKeyID)
	}
	if !options.RetainUntil.IsZero() {
		input.ObjectLockMode = types.ObjectLockModeCompliance
		input.ObjectLockRetainUntilDate = aws.Time(options.RetainUntil.UTC())
	}
	if options.LegalHold {
		input.ObjectLockLegalHoldStatus = types.ObjectLockLegalHoldStatusOn
	}
	out, err := s.client.PutObject(ctx, input)
	if err != nil {
		return ObjectReceipt{}, fmt.Errorf("s3 governed put: %w", err)
	}
	receipt := ObjectReceipt{Key: key}
	if out.VersionId != nil {
		receipt.VersionID = *out.VersionId
	}
	if out.ETag != nil {
		receipt.ETag = *out.ETag
	}
	if out.ChecksumSHA256 != nil {
		receipt.ChecksumSHA256 = *out.ChecksumSHA256
	}
	return receipt, nil
}

func (s *s3Store) GetVersion(ctx context.Context, key, versionID string) (io.ReadCloser, error) {
	if err := validateMultipartKey(key); err != nil || strings.TrimSpace(versionID) == "" {
		return nil, errors.New("object key and version id are required")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityVersioning); err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key, VersionId: &versionID})
	if err != nil {
		return nil, fmt.Errorf("s3 get object version: %w", err)
	}
	return out.Body, nil
}

func (s *s3Store) VersioningStatus(ctx context.Context) (string, error) {
	if err := RequireTargetTestedCapabilities(s, CapabilityVersioning); err != nil {
		return "", err
	}
	out, err := s.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: &s.bucket})
	if err != nil {
		return "", fmt.Errorf("s3 get versioning: %w", err)
	}
	return string(out.Status), nil
}

func (s *s3Store) EnableVersioning(ctx context.Context) error {
	if err := RequireTargetTestedCapabilities(s, CapabilityVersioning); err != nil {
		return err
	}
	_, err := s.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{Bucket: &s.bucket, VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled}})
	return err
}

func (s *s3Store) SetRetention(ctx context.Context, key, versionID string, mode types.ObjectLockRetentionMode, retainUntil time.Time) error {
	if err := validateMultipartKey(key); err != nil || strings.TrimSpace(versionID) == "" || retainUntil.IsZero() || !retainUntil.After(time.Now().UTC()) {
		return errors.New("valid object key, version id, and future retention time are required")
	}
	if mode != types.ObjectLockRetentionModeCompliance && mode != types.ObjectLockRetentionModeGovernance {
		return errors.New("unsupported object retention mode")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityVersioning, CapabilityObjectLock, CapabilityRetention); err != nil {
		return err
	}
	_, err := s.client.PutObjectRetention(ctx, &s3.PutObjectRetentionInput{Bucket: &s.bucket, Key: &key, VersionId: &versionID, Retention: &types.ObjectLockRetention{Mode: mode, RetainUntilDate: aws.Time(retainUntil.UTC())}})
	return err
}

func (s *s3Store) SetLegalHold(ctx context.Context, key, versionID string, enabled bool) error {
	if err := validateMultipartKey(key); err != nil || strings.TrimSpace(versionID) == "" {
		return errors.New("object key and version id are required")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityVersioning, CapabilityObjectLock, CapabilityLegalHold); err != nil {
		return err
	}
	status := types.ObjectLockLegalHoldStatusOff
	if enabled {
		status = types.ObjectLockLegalHoldStatusOn
	}
	_, err := s.client.PutObjectLegalHold(ctx, &s3.PutObjectLegalHoldInput{Bucket: &s.bucket, Key: &key, VersionId: &versionID, LegalHold: &types.ObjectLockLegalHold{Status: status}})
	return err
}
