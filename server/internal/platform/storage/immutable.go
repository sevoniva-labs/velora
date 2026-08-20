package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
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
	return ImmutableReceipt{Key: key, VersionID: *out.VersionId, SHA256: hex.EncodeToString(digest[:]), RetainUntil: retainUntil}, nil
}

func (s *s3Store) VerifyImmutable(ctx context.Context, receipt ImmutableReceipt) error {
	if receipt.Key == "" || receipt.VersionID == "" || receipt.SHA256 == "" || receipt.RetainUntil.IsZero() {
		return errors.New("immutable receipt is incomplete")
	}
	if err := RequireTargetTestedCapabilities(s, CapabilityChecksum, CapabilityVersioning, CapabilityObjectLock, CapabilityRetention); err != nil {
		return err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &receipt.Key, VersionId: &receipt.VersionID})
	if err != nil {
		return fmt.Errorf("s3 immutable head: %w", err)
	}
	if out.ObjectLockMode != types.ObjectLockModeCompliance || out.ObjectLockRetainUntilDate == nil || out.ObjectLockRetainUntilDate.Before(receipt.RetainUntil) {
		return errors.New("s3 object-lock compliance retention proof is missing or weaker than receipt")
	}
	if out.ChecksumSHA256 == nil {
		return errors.New("s3 immutable object checksum proof is missing")
	}
	digest, err := hex.DecodeString(receipt.SHA256)
	if err != nil || base64.StdEncoding.EncodeToString(digest) != *out.ChecksumSHA256 {
		return errors.New("s3 immutable object checksum proof does not match receipt")
	}
	return nil
}
