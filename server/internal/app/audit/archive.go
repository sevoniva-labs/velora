package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type ArchiveBatch struct {
	Events []Event
}

type ArchiveReceipt struct {
	Provider       string
	ObjectKey      string
	VersionID      string
	ContentHash    string
	EventCount     int
	ArchivedAt     time.Time
	ImmutableUntil time.Time
}

type WORMArchive interface {
	Archive(context.Context, ArchiveBatch, time.Time) (ArchiveReceipt, error)
	Verify(context.Context, ArchiveReceipt) error
	Provider() string
}

func DigestEvents(events []Event) (string, error) {
	payload, err := json.Marshal(events)
	if err != nil {
		return "", fmt.Errorf("encode audit archive batch: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateArchiveReceipt(batch ArchiveBatch, receipt ArchiveReceipt, now time.Time) error {
	if receipt.Provider == "" || receipt.ObjectKey == "" || receipt.VersionID == "" || receipt.ContentHash == "" {
		return errors.New("audit archive receipt is incomplete")
	}
	digest, err := DigestEvents(batch.Events)
	if err != nil {
		return err
	}
	if receipt.ContentHash != digest || receipt.EventCount != len(batch.Events) {
		return errors.New("audit archive receipt does not cover the archived batch")
	}
	if receipt.ImmutableUntil.IsZero() || !receipt.ImmutableUntil.After(now) {
		return errors.New("audit archive receipt has no future immutable retention")
	}
	return nil
}
