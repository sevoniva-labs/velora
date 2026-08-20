package audit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type Event struct {
	ID             string         `json:"id"`
	OccurredAt     time.Time      `json:"occurred_at"`
	RequestID      string         `json:"request_id"`
	OrganizationID string         `json:"organization_id,omitempty"`
	ActorID        string         `json:"actor_id,omitempty"`
	ActorName      string         `json:"actor_name,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resource_type,omitempty"`
	ResourceID     string         `json:"resource_id,omitempty"`
	Result         string         `json:"result"`
	ClientIP       string         `json:"client_ip,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
	SequenceNo     int64          `json:"sequence_no,omitempty"`
	PrevHash       string         `json:"prev_hash,omitempty"`
	EventHash      string         `json:"event_hash,omitempty"`
}

type Writer struct {
	db        *database.DB
	archive   WORMArchive
	forwarder Forwarder
}

var ErrIntegrityViolation = errors.New("audit integrity violation")

func NewWriter(db *database.DB) *Writer { return &Writer{db: db} }

func NewWriterWithArchive(db *database.DB, archive WORMArchive) *Writer {
	return &Writer{db: db, archive: archive}
}

type Forwarder interface {
	EnqueueTx(context.Context, *database.DB, *sql.Tx, Event) error
	Provider() string
}

func NewWriterWithForwarder(db *database.DB, forwarder Forwarder) *Writer {
	return &Writer{db: db, forwarder: forwarder}
}

func NewWriterWithArchiveAndForwarder(db *database.DB, archive WORMArchive, forwarder Forwarder) *Writer {
	return &Writer{db: db, archive: archive, forwarder: forwarder}
}

func (w *Writer) Write(ctx context.Context, e Event) error {
	if e.Result == "" {
		e.Result = "SUCCESS"
	}
	raw, err := json.Marshal(e.Details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}
	return w.db.WithinTx(ctx, func(txCtx context.Context) error {
		scope := e.OrganizationID
		if scope == "" {
			scope = "global"
		}
		insertHead := `INSERT INTO audit_chain_heads(scope,sequence_no,head_hash) VALUES(?,?,?)`
		if w.db.Provider == "postgres" {
			insertHead += ` ON CONFLICT (scope) DO NOTHING`
		} else {
			insertHead = `INSERT IGNORE INTO audit_chain_heads(scope,sequence_no,head_hash) VALUES(?,?,?)`
		}
		if _, err := w.db.ExecContext(txCtx, w.db.Rebind(insertHead), scope, int64(0), ""); err != nil {
			return err
		}
		var sequence int64
		var previous string
		if err := w.db.QueryRowContext(txCtx, w.db.Rebind(`SELECT sequence_no,head_hash FROM audit_chain_heads WHERE scope=? FOR UPDATE`), scope).Scan(&sequence, &previous); err != nil {
			return err
		}
		e.ID = uuid.NewString()
		e.OccurredAt = time.Now().UTC()
		e.SequenceNo = sequence + 1
		e.PrevHash = previous
		e.EventHash = auditEventHash(e, string(raw))
		if _, err := w.db.ExecContext(txCtx, w.db.Rebind(`INSERT INTO audit_logs(id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), e.ID, e.OccurredAt, e.RequestID, nullIfEmpty(e.OrganizationID), nullIfEmpty(e.ActorID), e.ActorName, e.Action, e.ResourceType, e.ResourceID, e.Result, e.ClientIP, string(raw), e.SequenceNo, e.PrevHash, e.EventHash); err != nil {
			return err
		}
		if w.forwarder != nil {
			tx, ok := database.TransactionFromContext(txCtx)
			if !ok {
				return errors.New("audit reliable forwarder requires active transaction")
			}
			if err := w.forwarder.EnqueueTx(txCtx, w.db, tx, e); err != nil {
				return fmt.Errorf("enqueue audit reliable event via %s: %w", w.forwarder.Provider(), err)
			}
		}
		_, err := w.db.ExecContext(txCtx, w.db.Rebind(`UPDATE audit_chain_heads SET sequence_no=?,head_hash=? WHERE scope=?`), e.SequenceNo, e.EventHash, scope)
		return err
	})
}

func (w *Writer) List(ctx context.Context, orgID string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, err := w.db.QueryContext(ctx, w.db.Rebind(`SELECT id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash FROM audit_logs WHERE organization_id=? ORDER BY occurred_at DESC LIMIT ?`), orgID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]Event, 0, limit)
	for rows.Next() {
		var e Event
		var org, actor sql.NullString
		var raw string
		var sequence sql.NullInt64
		var previous, eventHash sql.NullString
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &org, &actor, &e.ActorName, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.ClientIP, &raw, &sequence, &previous, &eventHash); err != nil {
			return nil, err
		}
		if sequence.Valid {
			e.SequenceNo = sequence.Int64
		}
		e.PrevHash, e.EventHash = previous.String, eventHash.String
		if org.Valid {
			e.OrganizationID = org.String
		}
		if actor.Valid {
			e.ActorID = actor.String
		}
		_ = json.Unmarshal([]byte(raw), &e.Details)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (w *Writer) VerifyIntegrity(ctx context.Context, orgID string) error {
	var expectedSequence int64 = 1
	previous := ""
	var anchorSequence int64
	var anchorHash string
	anchorErr := w.db.QueryRowContext(ctx, w.db.Rebind(`SELECT sequence_no,head_hash FROM audit_chain_anchors WHERE scope=?`), auditScope(orgID)).Scan(&anchorSequence, &anchorHash)
	anchored := false
	if anchorErr == nil {
		expectedSequence = anchorSequence + 1
		previous = anchorHash
		anchored = true
	} else if !errors.Is(anchorErr, sql.ErrNoRows) {
		return anchorErr
	}
	query, args := auditIntegrityQuery(orgID, anchorSequence, anchored)
	rows, err := w.db.QueryContext(ctx, w.db.Rebind(query), args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var e Event
		var organizationID, actorID sql.NullString
		var raw string
		var sequence sql.NullInt64
		var previousHash, eventHash sql.NullString
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &organizationID, &actorID, &e.ActorName, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.ClientIP, &raw, &sequence, &previousHash, &eventHash); err != nil {
			return err
		}
		if !sequence.Valid || !eventHash.Valid {
			return fmt.Errorf("%w: legacy audit event has no integrity proof", ErrIntegrityViolation)
		}
		e.OrganizationID, e.ActorID = organizationID.String, actorID.String
		e.SequenceNo, e.PrevHash, e.EventHash = sequence.Int64, previousHash.String, eventHash.String
		if e.SequenceNo != expectedSequence || e.PrevHash != previous || e.EventHash != auditEventHash(e, raw) {
			return fmt.Errorf("%w at sequence %d", ErrIntegrityViolation, e.SequenceNo)
		}
		previous = e.EventHash
		expectedSequence++
	}
	return rows.Err()
}

// auditIntegrityQuery starts at the first event after an immutable archive
// anchor. Archived rows remain in cold storage and must not be re-read as if
// they were part of the online chain prefix.
func auditIntegrityQuery(orgID string, anchorSequence int64, anchored bool) (string, []any) {
	query := `SELECT id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash FROM audit_logs WHERE (organization_id=? OR (?='' AND organization_id IS NULL))`
	args := []any{orgID, orgID}
	if anchored {
		query += ` AND sequence_no>?`
		args = append(args, anchorSequence)
	}
	query += ` ORDER BY sequence_no ASC,id ASC`
	return query, args
}

// PurgeExpired archives and deletes audit events older than retentionDays.
// It remains fail-closed until an immutable archive adapter is configured.
func (w *Writer) PurgeExpired(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	if w.archive == nil {
		return 0, errors.New("audit purge requires verified WORM archive adapter")
	}
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	events, err := w.expiredEvents(ctx, cutoff)
	if err != nil || len(events) == 0 {
		return 0, err
	}
	scopes := make(map[string]struct{}, len(events))
	anchors := make(map[string]Event, len(events))
	for _, event := range events {
		if event.SequenceNo <= 0 || event.EventHash == "" {
			return 0, fmt.Errorf("%w: expired audit event %s has no integrity proof", ErrIntegrityViolation, event.ID)
		}
		scope := auditScope(event.OrganizationID)
		scopes[scope] = struct{}{}
		if current, ok := anchors[scope]; !ok || event.SequenceNo > current.SequenceNo {
			anchors[scope] = event
		}
	}
	for scope := range scopes {
		orgID := scope
		if scope == "global" {
			orgID = ""
		}
		if err := w.VerifyIntegrity(ctx, orgID); err != nil {
			return 0, fmt.Errorf("verify audit scope %s before archive: %w", scope, err)
		}
	}
	batch := ArchiveBatch{Events: events}
	retainUntil := time.Now().UTC().Add(time.Duration(retentionDays) * 24 * time.Hour)
	receipt, err := w.archive.Archive(ctx, batch, retainUntil)
	if err != nil {
		return 0, fmt.Errorf("archive audit events: %w", err)
	}
	if err := ValidateArchiveReceipt(batch, receipt, time.Now().UTC()); err != nil {
		return 0, err
	}
	if err := w.archive.Verify(ctx, receipt); err != nil {
		return 0, fmt.Errorf("verify audit archive receipt: %w", err)
	}
	return w.deleteArchived(ctx, events, anchors, receipt)
}

func (w *Writer) expiredEvents(ctx context.Context, cutoff time.Time) ([]Event, error) {
	rows, err := w.db.QueryContext(ctx, w.db.Rebind(`SELECT id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash FROM audit_logs WHERE occurred_at<? ORDER BY organization_id,sequence_no,id`), cutoff)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	events := make([]Event, 0, 256)
	for rows.Next() {
		var event Event
		var org, actor sql.NullString
		var raw string
		var sequence sql.NullInt64
		var previous, eventHash sql.NullString
		if err := rows.Scan(&event.ID, &event.OccurredAt, &event.RequestID, &org, &actor, &event.ActorName, &event.Action, &event.ResourceType, &event.ResourceID, &event.Result, &event.ClientIP, &raw, &sequence, &previous, &eventHash); err != nil {
			return nil, err
		}
		event.OrganizationID, event.ActorID = org.String, actor.String
		event.SequenceNo, event.PrevHash, event.EventHash = sequence.Int64, previous.String, eventHash.String
		if err := json.Unmarshal([]byte(raw), &event.Details); err != nil {
			return nil, fmt.Errorf("decode audit details: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (w *Writer) deleteArchived(ctx context.Context, events []Event, anchors map[string]Event, receipt ArchiveReceipt) (deleted int64, err error) {
	err = w.db.WithinTx(ctx, func(txCtx context.Context) error {
		if receipt.ArchivedAt.IsZero() {
			receipt.ArchivedAt = time.Now().UTC()
		}
		_, err := w.db.ExecContext(txCtx, w.db.Rebind(`INSERT INTO audit_archive_receipts(id,provider,object_key,version_id,content_hash,event_count,archived_at,immutable_until) VALUES(?,?,?,?,?,?,?,?)`), uuid.NewString(), receipt.Provider, receipt.ObjectKey, receipt.VersionID, receipt.ContentHash, receipt.EventCount, receipt.ArchivedAt, receipt.ImmutableUntil)
		if err != nil {
			return err
		}
		for scope, event := range anchors {
			anchorQuery := `INSERT INTO audit_chain_anchors(scope,sequence_no,head_hash,archived_at) VALUES(?,?,?,?)`
			if w.db.Provider == "postgres" {
				anchorQuery += ` ON CONFLICT (scope) DO UPDATE SET sequence_no=EXCLUDED.sequence_no,head_hash=EXCLUDED.head_hash,archived_at=EXCLUDED.archived_at`
			} else {
				anchorQuery += ` ON DUPLICATE KEY UPDATE sequence_no=VALUES(sequence_no),head_hash=VALUES(head_hash),archived_at=VALUES(archived_at)`
			}
			if _, err := w.db.ExecContext(txCtx, w.db.Rebind(anchorQuery), scope, event.SequenceNo, event.EventHash, receipt.ArchivedAt); err != nil {
				return err
			}
		}
		for start := 0; start < len(events); start += 500 {
			end := start + 500
			if end > len(events) {
				end = len(events)
			}
			ids := events[start:end]
			placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
			args := make([]any, len(ids))
			for i, event := range ids {
				args[i] = event.ID
			}
			result, err := w.db.ExecContext(txCtx, w.db.Rebind("DELETE FROM audit_logs WHERE id IN ("+placeholders+")"), args...)
			if err != nil {
				return err
			}
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			deleted += n
		}
		return nil
	})
	return deleted, err
}

func auditScope(orgID string) string {
	if orgID == "" {
		return "global"
	}
	return orgID
}

func auditEventHash(e Event, raw string) string {
	digest := sha256.New()
	writeAuditHashPart(digest, e.ID)
	writeAuditHashPart(digest, e.OccurredAt.UTC().Format(time.RFC3339Nano))
	writeAuditHashPart(digest, e.RequestID)
	writeAuditHashPart(digest, e.OrganizationID)
	writeAuditHashPart(digest, e.ActorID)
	writeAuditHashPart(digest, e.ActorName)
	writeAuditHashPart(digest, e.Action)
	writeAuditHashPart(digest, e.ResourceType)
	writeAuditHashPart(digest, e.ResourceID)
	writeAuditHashPart(digest, e.Result)
	writeAuditHashPart(digest, e.ClientIP)
	writeAuditHashPart(digest, raw)
	writeAuditHashPart(digest, strconv.FormatInt(e.SequenceNo, 10))
	writeAuditHashPart(digest, e.PrevHash)
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func writeAuditHashPart(digest hash.Hash, value string) {
	_, _ = digest.Write([]byte(strconv.Itoa(len(value))))
	_, _ = digest.Write([]byte(":"))
	_, _ = digest.Write([]byte(value))
	_, _ = digest.Write([]byte("|"))
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
