package audit

import (
	"testing"
	"time"
)

func TestValidateArchiveReceiptRequiresExactImmutableEvidence(t *testing.T) {
	batch := ArchiveBatch{Events: []Event{{ID: "event-1", Action: "user.update", Result: "SUCCESS"}}}
	digest, err := DigestEvents(batch.Events)
	if err != nil {
		t.Fatal(err)
	}
	receipt := ArchiveReceipt{
		Provider:       "s3",
		ObjectKey:      "audit/event-1.json",
		VersionID:      "version-1",
		ContentHash:    digest,
		EventCount:     1,
		ArchivedAt:     time.Now().UTC(),
		ImmutableUntil: time.Now().UTC().Add(time.Hour),
	}
	if err := ValidateArchiveReceipt(batch, receipt, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	receipt.ContentHash = "tampered"
	if err := ValidateArchiveReceipt(batch, receipt, time.Now().UTC()); err == nil {
		t.Fatal("tampered archive receipt was accepted")
	}
}
