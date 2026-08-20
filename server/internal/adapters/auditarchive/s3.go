package auditarchive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	"github.com/sevoniva-labs/velora/server/internal/platform/storage"
)

type S3Archive struct {
	store  storage.ImmutableStore
	prefix string
}

func NewS3Archive(store storage.Store, prefix string) (*S3Archive, error) {
	immutable, ok := store.(storage.ImmutableStore)
	if !ok {
		return nil, errors.New("storage provider does not expose immutable object retention")
	}
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix != "" {
		prefix += "/"
	}
	return &S3Archive{store: immutable, prefix: prefix}, nil
}

func (a *S3Archive) Provider() string { return a.store.Provider() }

func (a *S3Archive) Archive(ctx context.Context, batch audit.ArchiveBatch, retainUntil time.Time) (audit.ArchiveReceipt, error) {
	if len(batch.Events) == 0 {
		return audit.ArchiveReceipt{}, errors.New("audit archive batch is empty")
	}
	payload, err := json.Marshal(batch.Events)
	if err != nil {
		return audit.ArchiveReceipt{}, fmt.Errorf("marshal audit archive: %w", err)
	}
	digest, err := audit.DigestEvents(batch.Events)
	if err != nil {
		return audit.ArchiveReceipt{}, err
	}
	key := fmt.Sprintf("%saudit/%s-%s.json", a.prefix, time.Now().UTC().Format("20060102T150405Z"), uuid.NewString())
	receipt, err := a.store.PutImmutable(ctx, key, payload, retainUntil)
	if err != nil {
		return audit.ArchiveReceipt{}, err
	}
	return audit.ArchiveReceipt{
		Provider:       a.Provider(),
		ObjectKey:      receipt.Key,
		VersionID:      receipt.VersionID,
		ContentHash:    digest,
		EventCount:     len(batch.Events),
		ArchivedAt:     time.Now().UTC(),
		ImmutableUntil: receipt.RetainUntil,
	}, nil
}

func (a *S3Archive) Verify(ctx context.Context, receipt audit.ArchiveReceipt) error {
	return a.store.VerifyImmutable(ctx, storage.ImmutableReceipt{
		Key: receipt.ObjectKey, VersionID: receipt.VersionID, SHA256: receipt.ContentHash, RetainUntil: receipt.ImmutableUntil,
	})
}
