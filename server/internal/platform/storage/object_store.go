package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ObjectStore is the vendor-neutral object contract used by new business
// modules. The legacy Store interface remains as a compatibility boundary for
// existing adapters and is wrapped by NewObjectStore.
type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, PutOptions) (ObjectInfo, error)
	Get(context.Context, string) (io.ReadCloser, ObjectInfo, error)
	Head(context.Context, string) (ObjectInfo, error)
	Delete(context.Context, string) error
	Capabilities(context.Context) (map[Capability]CapabilityStatus, error)
}

type PutOptions struct {
	ContentType    string
	ChecksumSHA256 string
	Metadata       map[string]string
	SSEMode        string
	SSEKMSKeyID    string
}

type ObjectInfo struct {
	Key            string
	ETag           string
	VersionID      string
	Size           int64
	ChecksumSHA256 string
	LastModified   time.Time
}

// NewObjectStore exposes the common contract without duplicating MinIO/COS
// implementations. The selected provider remains controlled by config and
// target capability evidence.
func NewObjectStore(store Store) (ObjectStore, error) {
	if store == nil {
		return nil, errors.New("object store is required")
	}
	return &objectStoreAdapter{store: store}, nil
}

type objectStoreAdapter struct{ store Store }

func (a *objectStoreAdapter) Put(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) (ObjectInfo, error) {
	if body == nil || size < -1 {
		return ObjectInfo{}, errors.New("object body and non-negative size are required")
	}
	key, err := normalizeObjectKey("", key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if s3Store, ok := a.store.(*s3Store); ok {
		return s3Store.putObject(ctx, key, body, size, opts)
	}
	if err := a.store.Put(ctx, key, body); err != nil {
		return ObjectInfo{}, err
	}
	return a.Head(ctx, key)
}

func (a *objectStoreAdapter) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	key, err := normalizeObjectKey("", key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	if s3Store, ok := a.store.(*s3Store); ok {
		return s3Store.getObject(ctx, key)
	}
	body, err := a.store.Get(ctx, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info, err := a.Head(ctx, key)
	if err != nil {
		_ = body.Close()
		return nil, ObjectInfo{}, err
	}
	return body, info, nil
}

func (a *objectStoreAdapter) Head(ctx context.Context, key string) (ObjectInfo, error) {
	key, err := normalizeObjectKey("", key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if s3Store, ok := a.store.(*s3Store); ok {
		return s3Store.headObject(ctx, key)
	}
	local, ok := a.store.(*local)
	if !ok {
		return ObjectInfo{}, errors.New("storage provider does not support head")
	}
	clean, err := local.path(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	info, err := os.Stat(filepath.Join(local.root, clean))
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: info.Size(), LastModified: info.ModTime().UTC()}, nil
}

func (a *objectStoreAdapter) Delete(ctx context.Context, key string) error {
	key, err := normalizeObjectKey("", key)
	if err != nil {
		return err
	}
	return a.store.Delete(ctx, key)
}

func (a *objectStoreAdapter) Capabilities(_ context.Context) (map[Capability]CapabilityStatus, error) {
	reporter, ok := a.store.(CapabilityReporter)
	if !ok {
		return nil, errors.New("storage provider does not report capabilities")
	}
	return reporter.Capabilities(), nil
}

func normalizePrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", nil
	}
	return normalizeObjectKey("", prefix)
}

func normalizeObjectKey(prefix, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || strings.HasPrefix(key, "/") || strings.ContainsRune(key, 0) || strings.ContainsRune(key, '\\') {
		return "", errors.New("object key must be a non-empty relative path")
	}
	segments := strings.Split(key, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", errors.New("object key contains an unsafe path segment")
		}
	}
	if prefix == "" {
		return strings.Join(segments, "/"), nil
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.Join(segments, "/"), nil
}

func (s *s3Store) putObject(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) (ObjectInfo, error) {
	key, err := s.objectKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	input := &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: body, Metadata: opts.Metadata}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	if opts.ChecksumSHA256 != "" {
		input.ChecksumAlgorithm = types.ChecksumAlgorithmSha256
		input.ChecksumSHA256 = aws.String(opts.ChecksumSHA256)
	}
	sseMode := strings.ToLower(strings.TrimSpace(opts.SSEMode))
	if sseMode == "" {
		sseMode = s.sseMode
	}
	sseKey := strings.TrimSpace(opts.SSEKMSKeyID)
	if sseKey == "" {
		sseKey = s.sseKMSID
	}
	switch sseMode {
	case "", "none":
	case "s3":
		input.ServerSideEncryption = types.ServerSideEncryptionAes256
	case "kms":
		if sseKey == "" {
			return ObjectInfo{}, errors.New("sse kms requires a key id")
		}
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(sseKey)
	default:
		return ObjectInfo{}, errors.New("unsupported server-side encryption mode")
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			seeker, ok := body.(io.Seeker)
			if !ok {
				break
			}
			if _, seekErr := seeker.Seek(0, io.SeekStart); seekErr != nil {
				return ObjectInfo{}, fmt.Errorf("reset object body for retry: %w", seekErr)
			}
		}
		out, callErr := s.client.PutObject(ctx, input)
		if callErr == nil {
			info := ObjectInfo{Key: key}
			if out.ETag != nil {
				info.ETag = *out.ETag
			}
			if out.VersionId != nil {
				info.VersionID = *out.VersionId
			}
			if out.ChecksumSHA256 != nil {
				info.ChecksumSHA256 = *out.ChecksumSHA256
			}
			return info, nil
		}
		lastErr = callErr
		if ctx.Err() != nil {
			break
		}
	}
	return ObjectInfo{}, lastErr
}

func (s *s3Store) getObject(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	key, err := s.objectKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return out.Body, ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength), ETag: aws.ToString(out.ETag), LastModified: aws.ToTime(out.LastModified)}, nil
}

func (s *s3Store) headObject(ctx context.Context, key string) (ObjectInfo, error) {
	key, err := s.objectKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength), ETag: aws.ToString(out.ETag), VersionID: aws.ToString(out.VersionId), ChecksumSHA256: aws.ToString(out.ChecksumSHA256), LastModified: aws.ToTime(out.LastModified)}, nil
}
