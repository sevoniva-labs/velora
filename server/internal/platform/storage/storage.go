package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	appcfg "github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/tlsx"
)

type Store interface {
	Put(context.Context, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Ping(context.Context) error
	Provider() string
}

type ProfileReporter interface {
	Profile() ProviderProfile
}

type Capability string

const (
	CapabilityBasicObjectIO       Capability = "basic_object_io"
	CapabilityMultipartRecovery   Capability = "multipart_recovery"
	CapabilityChecksum            Capability = "checksum"
	CapabilitySSES3               Capability = "sse_s3"
	CapabilitySSEKMS              Capability = "sse_kms"
	CapabilityVersioning          Capability = "versioning"
	CapabilityObjectLock          Capability = "object_lock"
	CapabilityRetention           Capability = "retention"
	CapabilityLegalHold           Capability = "legal_hold"
	CapabilityConstrainedPresign  Capability = "constrained_presign"
	CapabilityTemporaryCredential Capability = "temporary_credentials"
)

type CapabilityState string

const (
	CapabilitySupported   CapabilityState = "supported"
	CapabilityUnsupported CapabilityState = "unsupported"
	CapabilityUnknown     CapabilityState = "unknown"
)

type CapabilityStatus struct {
	State    CapabilityState `json:"state"`
	Evidence string          `json:"evidence"`
}

type CapabilityReporter interface {
	Capabilities() map[Capability]CapabilityStatus
}

// RequireCapabilities fails closed. A provider must be contract-tested before
// an operation can rely on an advanced S3 feature.
func RequireCapabilities(store Store, required ...Capability) error {
	reporter, ok := store.(CapabilityReporter)
	if !ok {
		return errors.New("storage provider does not report capabilities")
	}
	capabilities := reporter.Capabilities()
	for _, capability := range required {
		status, present := capabilities[capability]
		if !present || status.State != CapabilitySupported {
			return fmt.Errorf("storage capability %q is not verified", capability)
		}
	}
	return nil
}

func New(ctx context.Context, c appcfg.Storage) (Store, error) {
	profile, err := ResolveProviderProfile(c.Provider)
	if err != nil {
		return nil, err
	}
	switch profile {
	case ProviderProfileLocal:
		if err := os.MkdirAll(c.LocalRoot, 0o750); err != nil {
			return nil, err
		}
		return &local{root: c.LocalRoot}, nil
	default:
		contract := defaultCapabilityContract(profile)
		if path := strings.TrimSpace(c.CapabilityContractFile); path != "" {
			loaded, err := LoadCapabilityContract(path)
			if err != nil {
				return nil, err
			}
			if loaded.Profile != profile || loaded.Target != TargetForConfig(c) {
				return nil, errors.New("storage capability contract does not match the configured target")
			}
			contract = loaded
		}
		return newS3(ctx, c, profile, contract)
	}
}

// TargetForConfig returns the stable target binding used by capability
// evidence. A contract must never be reused for a different endpoint, bucket,
// region, or object prefix.
func TargetForConfig(c appcfg.Storage) string {
	return strings.Join([]string{strings.TrimRight(strings.TrimSpace(c.Endpoint), "/"), strings.TrimSpace(c.Region), strings.TrimSpace(c.Bucket), strings.Trim(strings.TrimSpace(c.Prefix), "/")}, "|")
}

// NewWithCapabilityContract enables advanced S3 operations only after the
// configured target has passed an immutable, target-specific contract test.
func NewWithCapabilityContract(ctx context.Context, c appcfg.Storage, contract CapabilityContract) (Store, error) {
	profile, err := ResolveProviderProfile(c.Provider)
	if err != nil {
		return nil, err
	}
	if profile != contract.Profile {
		return nil, fmt.Errorf("storage capability contract profile %q does not match configured profile %q", contract.Profile, profile)
	}
	if err := contract.Validate(CapabilityBasicObjectIO); err != nil {
		return nil, err
	}
	return newS3(ctx, c, profile, contract)
}

type local struct{ root string }

func (l *local) path(key string) (string, error) {
	clean, err := normalizeObjectKey("", key)
	if err != nil {
		return "", err
	}
	clean = filepath.Clean(clean)
	return clean, nil
}
func (l *local) Put(_ context.Context, key string, r io.Reader) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	root, e := os.OpenRoot(l.root)
	if e != nil {
		return e
	}
	defer func() { _ = root.Close() }()
	if e = root.MkdirAll(filepath.Dir(p), 0o750); e != nil {
		return e
	}
	f, e := root.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if e != nil {
		return e
	}
	if _, e = io.Copy(f, r); e != nil {
		_ = f.Close()
		return e
	}
	return f.Close()
}
func (l *local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, e := l.path(key)
	if e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(l.root)
	if e != nil {
		return nil, e
	}
	f, e := root.Open(p)
	closeErr := root.Close()
	if e != nil {
		return nil, e
	}
	if closeErr != nil {
		_ = f.Close()
		return nil, closeErr
	}
	return f, nil
}
func (l *local) Delete(_ context.Context, key string) error {
	p, e := l.path(key)
	if e != nil {
		return e
	}
	root, e := os.OpenRoot(l.root)
	if e != nil {
		return e
	}
	defer func() { _ = root.Close() }()
	return root.Remove(p)
}
func (l *local) Ping(context.Context) error { return nil }
func (l *local) Provider() string           { return "local" }
func (l *local) Profile() ProviderProfile   { return ProviderProfileLocal }
func (l *local) Capabilities() map[Capability]CapabilityStatus {
	return map[Capability]CapabilityStatus{
		CapabilityBasicObjectIO:     {State: CapabilitySupported, Evidence: "local filesystem contract"},
		CapabilityMultipartRecovery: {State: CapabilityUnsupported, Evidence: "local provider has no multipart protocol"},
	}
}

type s3Store struct {
	client   *s3.Client
	presign  *s3.PresignClient
	bucket   string
	prefix   string
	sseMode  string
	sseKMSID string
	profile  ProviderProfile
	contract CapabilityContract
}

func newS3(ctx context.Context, c appcfg.Storage, profile ProviderProfile, contract CapabilityContract) (Store, error) {
	if c.TLS && strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.Endpoint)), "http://") {
		return nil, errors.New("storage tls=true requires https endpoint")
	}
	tlsCfg, err := tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled: c.TLS, CAFile: c.TLSCAFile, CertFile: c.TLSCertFile, KeyFile: c.TLSKeyFile, ServerName: c.TLSServerName,
	})
	if err != nil {
		return nil, err
	}
	opts := []func(*config.LoadOptions) error{config.WithRegion(c.Region)}
	if tlsCfg != nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = tlsCfg
		opts = append(opts, config.WithHTTPClient(&http.Client{Transport: tr}))
	}
	if c.AccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.SessionToken)))
	}
	cfg, e := config.LoadDefaultConfig(ctx, opts...)
	if e != nil {
		return nil, e
	}
	cli := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = c.PathStyle
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
	})
	prefix, err := normalizePrefix(c.Prefix)
	if err != nil {
		return nil, err
	}
	return &s3Store{client: cli, presign: s3.NewPresignClient(cli), bucket: c.Bucket, prefix: prefix, sseMode: strings.ToLower(strings.TrimSpace(c.SSEMode)), sseKMSID: strings.TrimSpace(c.SSEKMSKeyID), profile: profile, contract: contract}, nil
}
func (s *s3Store) Put(ctx context.Context, key string, r io.Reader) error {
	if r == nil {
		return errors.New("storage put body is required")
	}
	key, err := s.objectKey(key)
	if err != nil {
		return err
	}
	input := &s3.PutObjectInput{
		Bucket: &s.bucket, Key: &key, Body: r,
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
	}
	switch s.sseMode {
	case "s3":
		input.ServerSideEncryption = types.ServerSideEncryptionAes256
	case "kms":
		if strings.TrimSpace(s.sseKMSID) == "" {
			return errors.New("storage sse kms key id is required")
		}
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(s.sseKMSID)
	}
	_, e := s.client.PutObject(ctx, input)
	return e
}
func (s *s3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	key, err := s.objectKey(key)
	if err != nil {
		return nil, err
	}
	o, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return nil, e
	}
	return o.Body, nil
}
func (s *s3Store) Delete(ctx context.Context, key string) error {
	key, err := s.objectKey(key)
	if err != nil {
		return err
	}
	_, e := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return e
}
func (s *s3Store) Ping(ctx context.Context) error {
	_, e := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &s.bucket})
	if e != nil {
		return fmt.Errorf("s3 head bucket: %w", e)
	}
	return nil
}
func (s *s3Store) Provider() string         { return "s3" }
func (s *s3Store) Profile() ProviderProfile { return s.profile }
func (s *s3Store) CapabilityContract() CapabilityContract {
	contract := s.contract
	contract.Capabilities = cloneCapabilities(contract.Capabilities)
	return contract
}
func (s *s3Store) Capabilities() map[Capability]CapabilityStatus {
	return cloneCapabilities(s.contract.Capabilities)
}

func (s *s3Store) objectKey(key string) (string, error) {
	return normalizeObjectKey(s.prefix, key)
}

func cloneCapabilities(input map[Capability]CapabilityStatus) map[Capability]CapabilityStatus {
	output := make(map[Capability]CapabilityStatus, len(input))
	for capability, status := range input {
		output[capability] = status
	}
	return output
}
