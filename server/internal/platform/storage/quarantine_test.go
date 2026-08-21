package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestQuarantineRequiresCleanEvidenceBeforePromotion(t *testing.T) {
	objects := &fakeQuarantineObjects{items: map[string][]byte{}}
	records := &fakeQuarantineRecords{items: map[string]QuarantineRecord{}}
	target := &fakeGovernanceStore{}
	controller, err := NewQuarantineController(objects, records, fakeScanner{evidence: ScanEvidence{Scanner: "scanner", EvidenceID: "evidence-1", Verdict: ScanVerdictClean, CompletedAt: time.Now().UTC()}}, target, 1024, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	record, err := controller.Submit(context.Background(), QuarantineRequest{Key: "docs/report.pdf", Payload: []byte("plain text")})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := controller.Promote(context.Background(), record.ID); err == nil {
		t.Fatal("promoted before clean scan")
	}
	if _, err := controller.Scan(context.Background(), record.ID); err != nil {
		t.Fatal(err)
	}
	receipt, promoted, err := controller.Promote(context.Background(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != QuarantineStatusPromoted || receipt.Key != record.Key || target.options.ChecksumSHA256 == "" {
		t.Fatalf("unexpected promotion result: %#v %#v", receipt, promoted)
	}
}

func TestQuarantineRejectsMaliciousEvidenceAndTypeMismatch(t *testing.T) {
	objects := &fakeQuarantineObjects{items: map[string][]byte{}}
	records := &fakeQuarantineRecords{items: map[string]QuarantineRecord{}}
	target := &fakeGovernanceStore{}
	controller, err := NewQuarantineController(objects, records, fakeScanner{evidence: ScanEvidence{Scanner: "scanner", EvidenceID: "evidence-2", Verdict: ScanVerdictMalicious, CompletedAt: time.Now().UTC()}}, target, 1024, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Submit(context.Background(), QuarantineRequest{Key: "docs/report.pdf", ContentType: "image/png", Payload: []byte("plain text")}); err == nil {
		t.Fatal("accepted content type mismatch")
	}
	record, err := controller.Submit(context.Background(), QuarantineRequest{Key: "docs/report.pdf", Payload: []byte("plain text")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.Scan(context.Background(), record.ID); err == nil {
		t.Fatal("accepted malicious evidence")
	}
	if records.items[record.ID].Status != QuarantineStatusRejected {
		t.Fatalf("unexpected scan status: %s", records.items[record.ID].Status)
	}
}

type fakeScanner struct{ evidence ScanEvidence }

func (f fakeScanner) Scan(context.Context, string, io.Reader) (ScanEvidence, error) {
	return f.evidence, nil
}

type fakeQuarantineObjects struct{ items map[string][]byte }

func (f *fakeQuarantineObjects) Put(_ context.Context, id string, payload []byte) error {
	f.items[id] = append([]byte(nil), payload...)
	return nil
}
func (f *fakeQuarantineObjects) Get(_ context.Context, id string) (io.ReadCloser, error) {
	payload, ok := f.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}
func (f *fakeQuarantineObjects) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

type fakeQuarantineRecords struct{ items map[string]QuarantineRecord }

func (f *fakeQuarantineRecords) Create(_ context.Context, record QuarantineRecord) error {
	f.items[record.ID] = record
	return nil
}
func (f *fakeQuarantineRecords) Get(_ context.Context, id string) (QuarantineRecord, error) {
	record, ok := f.items[id]
	if !ok {
		return QuarantineRecord{}, errors.New("not found")
	}
	return record, nil
}
func (f *fakeQuarantineRecords) CompareAndSwap(_ context.Context, id string, expected QuarantineStatus, next QuarantineRecord) error {
	current, ok := f.items[id]
	if !ok || current.Status != expected {
		return errors.New("state conflict")
	}
	f.items[id] = next
	return nil
}

type fakeGovernanceStore struct{ options ObjectWriteOptions }

func (f *fakeGovernanceStore) Put(context.Context, string, io.Reader) error { return nil }
func (f *fakeGovernanceStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeGovernanceStore) Delete(context.Context, string) error { return nil }
func (f *fakeGovernanceStore) Ping(context.Context) error           { return nil }
func (f *fakeGovernanceStore) Provider() string                     { return "fake" }
func (f *fakeGovernanceStore) PutGoverned(_ context.Context, key string, _ []byte, options ObjectWriteOptions) (ObjectReceipt, error) {
	f.options = options
	return ObjectReceipt{Key: key, VersionID: "version-1"}, nil
}
func (f *fakeGovernanceStore) GetVersion(context.Context, string, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeGovernanceStore) VersioningStatus(context.Context) (string, error) {
	return "", errors.New("not implemented")
}
func (f *fakeGovernanceStore) EnableVersioning(context.Context) error { return nil }
func (f *fakeGovernanceStore) SetRetention(context.Context, string, string, types.ObjectLockRetentionMode, time.Time) error {
	return nil
}
func (f *fakeGovernanceStore) SetLegalHold(context.Context, string, string, bool) error { return nil }
