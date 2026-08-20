package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type QuarantineStatus string

const (
	QuarantineStatusQuarantined    QuarantineStatus = "Quarantined"
	QuarantineStatusScanning       QuarantineStatus = "Scanning"
	QuarantineStatusClean          QuarantineStatus = "Clean"
	QuarantineStatusScanFailed     QuarantineStatus = "ScanFailed"
	QuarantineStatusRejected       QuarantineStatus = "Rejected"
	QuarantineStatusExpired        QuarantineStatus = "Expired"
	QuarantineStatusPromoted       QuarantineStatus = "Promoted"
	QuarantineStatusPromotionFail  QuarantineStatus = "PromotionFailed"
	QuarantineStatusCleanupPending QuarantineStatus = "CleanupPending"
)

type ScanVerdict string

const (
	ScanVerdictClean     ScanVerdict = "Clean"
	ScanVerdictMalicious ScanVerdict = "Malicious"
)

type ScanEvidence struct {
	Scanner     string
	EvidenceID  string
	Verdict     ScanVerdict
	CompletedAt time.Time
}

type QuarantineRecord struct {
	ID                  string
	Key                 string
	ContentType         string
	DetectedContentType string
	Size                int64
	SHA256              string
	Status              QuarantineStatus
	CreatedAt           time.Time
	ExpiresAt           time.Time
	Scan                ScanEvidence
	PromotionOptions    ObjectWriteOptions
	PromotedVersionID   string
}

type QuarantineRequest struct {
	Key                 string
	ContentType         string
	Payload             []byte
	MaxBytes            int64
	AllowedContentTypes []string
	PromotionOptions    ObjectWriteOptions
}

type QuarantineObjectStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

// QuarantineRecordStore must implement CompareAndSwap atomically in its
// persistent adapter. This prevents two workers from scanning or promoting
// the same regulated object concurrently.
type QuarantineRecordStore interface {
	Create(context.Context, QuarantineRecord) error
	Get(context.Context, string) (QuarantineRecord, error)
	CompareAndSwap(context.Context, string, QuarantineStatus, QuarantineRecord) error
}

type MalwareScanner interface {
	Scan(context.Context, string, io.Reader) (ScanEvidence, error)
}

type QuarantineController struct {
	objects QuarantineObjectStore
	records QuarantineRecordStore
	scanner MalwareScanner
	target  GovernanceStore
	maxSize int64
	ttl     time.Duration
	now     func() time.Time
}

func NewQuarantineController(objects QuarantineObjectStore, records QuarantineRecordStore, scanner MalwareScanner, target GovernanceStore, maxSize int64, ttl time.Duration) (*QuarantineController, error) {
	if objects == nil || records == nil || scanner == nil || target == nil {
		return nil, errors.New("quarantine dependencies are required")
	}
	if maxSize <= 0 || ttl <= 0 {
		return nil, errors.New("quarantine max size and ttl must be positive")
	}
	return &QuarantineController{objects: objects, records: records, scanner: scanner, target: target, maxSize: maxSize, ttl: ttl, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (c *QuarantineController) Submit(ctx context.Context, request QuarantineRequest) (QuarantineRecord, error) {
	if err := validateMultipartKey(request.Key); err != nil {
		return QuarantineRecord{}, err
	}
	if len(request.Payload) == 0 {
		return QuarantineRecord{}, errors.New("empty upload is not allowed")
	}
	if int64(len(request.Payload)) > c.maxSize || (request.MaxBytes > 0 && int64(len(request.Payload)) > request.MaxBytes) {
		return QuarantineRecord{}, errors.New("upload exceeds quarantine size limit")
	}
	digest := sha256.Sum256(request.Payload)
	checksum := base64.StdEncoding.EncodeToString(digest[:])
	if request.PromotionOptions.ChecksumSHA256 != "" && request.PromotionOptions.ChecksumSHA256 != checksum {
		return QuarantineRecord{}, errors.New("promotion checksum does not match upload")
	}
	request.PromotionOptions.ChecksumSHA256 = checksum
	if err := validateGovernedWrite(request.Key, request.PromotionOptions); err != nil {
		return QuarantineRecord{}, err
	}
	detected := http.DetectContentType(request.Payload)
	declared := normalizeContentType(request.ContentType)
	if declared == "" {
		declared = detected
	}
	if declared != "application/octet-stream" && detected != "application/octet-stream" && declared != detected {
		return QuarantineRecord{}, errors.New("declared content type does not match detected content")
	}
	if !contentTypeAllowed(declared, request.AllowedContentTypes) {
		return QuarantineRecord{}, errors.New("content type is not allowed")
	}

	id, err := newQuarantineID()
	if err != nil {
		return QuarantineRecord{}, err
	}
	now := c.now().UTC()
	record := QuarantineRecord{
		ID:                  id,
		Key:                 request.Key,
		ContentType:         declared,
		DetectedContentType: detected,
		Size:                int64(len(request.Payload)),
		SHA256:              hex.EncodeToString(digest[:]),
		Status:              QuarantineStatusQuarantined,
		CreatedAt:           now,
		ExpiresAt:           now.Add(c.ttl),
		PromotionOptions:    request.PromotionOptions,
	}
	if err := c.objects.Put(ctx, id, request.Payload); err != nil {
		return QuarantineRecord{}, fmt.Errorf("store quarantined object: %w", err)
	}
	if err := c.records.Create(ctx, record); err != nil {
		_ = c.objects.Delete(ctx, id)
		return QuarantineRecord{}, fmt.Errorf("create quarantine record: %w", err)
	}
	return record, nil
}

func (c *QuarantineController) Scan(ctx context.Context, id string) (QuarantineRecord, error) {
	record, err := c.records.Get(ctx, id)
	if err != nil {
		return QuarantineRecord{}, err
	}
	if c.now().UTC().After(record.ExpiresAt) {
		expired, transitionErr := c.transition(ctx, record, QuarantineStatusExpired, func(QuarantineRecord) QuarantineRecord { return record })
		if transitionErr != nil {
			return QuarantineRecord{}, transitionErr
		}
		return expired, errors.New("quarantine object has expired")
	}
	if record.Status != QuarantineStatusQuarantined {
		return QuarantineRecord{}, fmt.Errorf("quarantine object is not scannable: %s", record.Status)
	}
	record, err = c.transition(ctx, record, QuarantineStatusScanning, func(next QuarantineRecord) QuarantineRecord { return next })
	if err != nil {
		return QuarantineRecord{}, err
	}
	body, err := c.objects.Get(ctx, id)
	if err != nil {
		return c.failScan(ctx, record, fmt.Errorf("read quarantined object: %w", err))
	}
	evidence, scanErr := c.scanner.Scan(ctx, record.Key, body)
	closeErr := body.Close()
	if scanErr != nil {
		return c.failScan(ctx, record, fmt.Errorf("malware scan: %w", scanErr))
	}
	if closeErr != nil {
		return c.failScan(ctx, record, fmt.Errorf("close quarantined object: %w", closeErr))
	}
	if evidence.Verdict == ScanVerdictMalicious {
		rejected, transitionErr := c.transition(ctx, record, QuarantineStatusRejected, func(next QuarantineRecord) QuarantineRecord {
			next.Scan = evidence
			return next
		})
		if transitionErr != nil {
			return QuarantineRecord{}, transitionErr
		}
		return rejected, errors.New("malware scan detected malicious content")
	}
	if evidence.Scanner == "" || evidence.EvidenceID == "" || evidence.CompletedAt.IsZero() || evidence.Verdict != ScanVerdictClean {
		return c.failScan(ctx, record, errors.New("malware scan did not produce clean evidence"))
	}
	return c.transition(ctx, record, QuarantineStatusClean, func(next QuarantineRecord) QuarantineRecord { return recordWithScan(next, evidence) })
}

func (c *QuarantineController) Promote(ctx context.Context, id string) (ObjectReceipt, QuarantineRecord, error) {
	record, err := c.records.Get(ctx, id)
	if err != nil {
		return ObjectReceipt{}, QuarantineRecord{}, err
	}
	if c.now().UTC().After(record.ExpiresAt) {
		_, transitionErr := c.transition(ctx, record, QuarantineStatusExpired, func(QuarantineRecord) QuarantineRecord { return record })
		if transitionErr != nil {
			return ObjectReceipt{}, QuarantineRecord{}, transitionErr
		}
		return ObjectReceipt{}, record, errors.New("quarantine object has expired")
	}
	if record.Status != QuarantineStatusClean {
		return ObjectReceipt{}, record, fmt.Errorf("quarantine object is not promotable: %s", record.Status)
	}
	body, err := c.objects.Get(ctx, id)
	if err != nil {
		return ObjectReceipt{}, record, fmt.Errorf("read quarantined object: %w", err)
	}
	payload, readErr := io.ReadAll(io.LimitReader(body, record.Size+1))
	closeErr := body.Close()
	if readErr != nil || closeErr != nil || int64(len(payload)) != record.Size {
		return ObjectReceipt{}, record, errors.New("quarantined object could not be read completely")
	}
	digest := sha256.Sum256(payload)
	if hex.EncodeToString(digest[:]) != record.SHA256 {
		_, _ = c.transition(ctx, record, QuarantineStatusRejected, func(QuarantineRecord) QuarantineRecord { return record })
		return ObjectReceipt{}, record, errors.New("quarantined object checksum changed")
	}
	receipt, err := c.target.PutGoverned(ctx, record.Key, payload, record.PromotionOptions)
	if err != nil {
		failed, transitionErr := c.transition(ctx, record, QuarantineStatusPromotionFail, func(QuarantineRecord) QuarantineRecord { return record })
		if transitionErr != nil {
			return ObjectReceipt{}, record, fmt.Errorf("promote quarantined object: %w; record state: %v", err, transitionErr)
		}
		return ObjectReceipt{}, failed, fmt.Errorf("promote quarantined object: %w", err)
	}
	record.PromotedVersionID = receipt.VersionID
	if err := c.objects.Delete(ctx, id); err != nil {
		pending, transitionErr := c.transition(ctx, record, QuarantineStatusCleanupPending, func(QuarantineRecord) QuarantineRecord { return record })
		if transitionErr != nil {
			return receipt, record, fmt.Errorf("cleanup quarantined object: %w; record state: %v", err, transitionErr)
		}
		return receipt, pending, fmt.Errorf("cleanup quarantined object: %w", err)
	}
	final, err := c.transition(ctx, record, QuarantineStatusPromoted, func(QuarantineRecord) QuarantineRecord { return record })
	if err != nil {
		return receipt, record, err
	}
	return receipt, final, nil
}

func (c *QuarantineController) Cleanup(ctx context.Context, id string) (QuarantineRecord, error) {
	record, err := c.records.Get(ctx, id)
	if err != nil {
		return QuarantineRecord{}, err
	}
	if record.Status != QuarantineStatusCleanupPending {
		return QuarantineRecord{}, fmt.Errorf("quarantine object is not awaiting cleanup: %s", record.Status)
	}
	if err := c.objects.Delete(ctx, id); err != nil {
		return QuarantineRecord{}, err
	}
	return c.transition(ctx, record, QuarantineStatusPromoted, func(QuarantineRecord) QuarantineRecord { return record })
}

func (c *QuarantineController) failScan(ctx context.Context, record QuarantineRecord, cause error) (QuarantineRecord, error) {
	failed, transitionErr := c.transition(ctx, record, QuarantineStatusScanFailed, func(QuarantineRecord) QuarantineRecord { return record })
	if transitionErr != nil {
		return QuarantineRecord{}, fmt.Errorf("%w; record state: %v", cause, transitionErr)
	}
	return failed, cause
}

func (c *QuarantineController) transition(ctx context.Context, record QuarantineRecord, status QuarantineStatus, mutate func(QuarantineRecord) QuarantineRecord) (QuarantineRecord, error) {
	next := mutate(record)
	next.Status = status
	if err := c.records.CompareAndSwap(ctx, record.ID, record.Status, next); err != nil {
		return QuarantineRecord{}, fmt.Errorf("quarantine state transition %s -> %s: %w", record.Status, status, err)
	}
	return next, nil
}

func recordWithScan(record QuarantineRecord, evidence ScanEvidence) QuarantineRecord {
	record.Scan = evidence
	return record
}

func newQuarantineID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate quarantine id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func normalizeContentType(value string) string {
	return strings.TrimSpace(strings.SplitN(value, ";", 2)[0])
}

func contentTypeAllowed(value string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	value = normalizeContentType(value)
	for _, candidate := range allowed {
		candidate = normalizeContentType(candidate)
		if candidate == value || (strings.HasSuffix(candidate, "/*") && strings.HasPrefix(value, strings.TrimSuffix(candidate, "*"))) {
			return true
		}
	}
	return false
}
