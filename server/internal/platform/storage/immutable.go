package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type ImmutableReceipt struct {
	Key         string
	VersionID   string
	SHA256      string
	RetainUntil time.Time
}

// ImmutableStore is intentionally separate from Store. A provider must expose
// and verify object-lock metadata before it can be used for WORM retention.
type ImmutableStore interface {
	Store
	PutImmutable(context.Context, string, []byte, time.Time) (ImmutableReceipt, error)
	VerifyImmutable(context.Context, ImmutableReceipt) error
}

func (s *s3Store) PutImmutable(ctx context.Context, key string, payload []byte, retainUntil time.Time) (ImmutableReceipt, error) {
	if err := validateMultipartKey(key); err != nil {
		return ImmutableReceipt{}, err
	}
	logicalKey := key
	key, err := s.objectKey(key)
	if err != nil {
		return ImmutableReceipt{}, err
	}
	// S3 Object Lock timestamps have second precision. Normalize the receipt to
	// the value the target can actually return, otherwise a nanosecond-level
	// comparison would reject an otherwise valid retained object.
	retainUntil = retainUntil.UTC().Truncate(time.Second)
	if retainUntil.IsZero() || !retainUntil.After(time.Now().UTC()) {
		return ImmutableReceipt{}, errors.New("immutable object retention must be in the future")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityChecksum, CapabilityVersioning, CapabilityObjectLock, CapabilityRetention); err != nil {
		return ImmutableReceipt{}, err
	}
	digest := sha256.Sum256(payload)
	checksum := base64.StdEncoding.EncodeToString(digest[:])
	out, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:                    &s.bucket,
		Key:                       &key,
		Body:                      bytes.NewReader(payload),
		ChecksumAlgorithm:         types.ChecksumAlgorithmSha256,
		ChecksumSHA256:            aws.String(checksum),
		ObjectLockMode:            types.ObjectLockModeCompliance,
		ObjectLockRetainUntilDate: &retainUntil,
		ContentType:               aws.String("application/json"),
	})
	if err != nil {
		return ImmutableReceipt{}, fmt.Errorf("s3 immutable put: %w", err)
	}
	if out.VersionId == nil || *out.VersionId == "" {
		return ImmutableReceipt{}, errors.New("s3 immutable put did not return a version id")
	}
	return ImmutableReceipt{Key: logicalKey, VersionID: *out.VersionId, SHA256: hex.EncodeToString(digest[:]), RetainUntil: retainUntil}, nil
}

func (s *s3Store) VerifyImmutable(ctx context.Context, receipt ImmutableReceipt) error {
	if receipt.Key == "" || receipt.VersionID == "" || receipt.SHA256 == "" || receipt.RetainUntil.IsZero() {
		return errors.New("immutable receipt is incomplete")
	}
	key, err := s.objectKey(receipt.Key)
	if err != nil {
		return err
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityChecksum, CapabilityVersioning, CapabilityObjectLock, CapabilityRetention); err != nil {
		return err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key, VersionId: &receipt.VersionID})
	if err != nil {
		return fmt.Errorf("s3 immutable head: %w", err)
	}
	if out.ObjectLockMode != types.ObjectLockModeCompliance || out.ObjectLockRetainUntilDate == nil || out.ObjectLockRetainUntilDate.Before(receipt.RetainUntil) {
		return errors.New("s3 object-lock compliance retention proof is missing or weaker than receipt")
	}
	if out.ChecksumSHA256 != nil {
		digest, err := hex.DecodeString(receipt.SHA256)
		if err != nil || base64.StdEncoding.EncodeToString(digest) != *out.ChecksumSHA256 {
			return errors.New("s3 immutable object checksum proof does not match receipt")
		}
		return nil
	}
	// Some S3-compatible targets (including MinIO versions without checksum
	// response headers) accept a checksum on PUT but omit it from HEAD. Read the
	// retained version and hash the bytes as a portable integrity fallback.
	got, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key, VersionId: &receipt.VersionID})
	if err != nil {
		return fmt.Errorf("s3 immutable checksum fallback get: %w", err)
	}
	defer func() { _ = got.Body.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, got.Body); err != nil {
		return fmt.Errorf("s3 immutable checksum fallback read: %w", err)
	}
	if hex.EncodeToString(h.Sum(nil)) != receipt.SHA256 {
		return errors.New("s3 immutable object checksum fallback does not match receipt")
	}
	return nil
}
