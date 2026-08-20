package idempotency

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type Record struct {
	ID             string
	RequestHash    string
	State          string
	ResponseStatus int
	ResponseBody   []byte
	ExpiresAt      time.Time
}

type BeginResult struct {
	Created  bool
	Conflict bool
	Record   Record
}

type Store struct{ db *database.DB }

func New(db *database.DB) *Store { return &Store{db: db} }

// Begin creates a cross-pod idempotency reservation. A duplicate key with a
// different request hash is a conflict and must never replay the old result.
func (s *Store) Begin(ctx context.Context, orgID, scope, key, requestHash string, ttl time.Duration) (BeginResult, error) {
	if orgID == "" || scope == "" || key == "" || requestHash == "" {
		return BeginResult{}, errors.New("idempotency: missing required field")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := time.Now().UTC()
	r := Record{ID: uuid.NewString(), RequestHash: requestHash, State: "PROCESSING", ExpiresAt: now.Add(ttl)}
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`INSERT INTO idempotency_records(id,organization_id,scope,idem_key,request_hash,state,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?)`), r.ID, orgID, scope, key, requestHash, r.State, now, r.ExpiresAt)
	if err == nil {
		return BeginResult{Created: true, Record: r}, nil
	}

	var status sql.NullInt64
	var body []byte
	err2 := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT id,request_hash,state,response_status,response_body,expires_at FROM idempotency_records WHERE organization_id=? AND scope=? AND idem_key=?`), orgID, scope, key).Scan(&r.ID, &r.RequestHash, &r.State, &status, &body, &r.ExpiresAt)
	if err2 != nil {
		return BeginResult{}, err
	}
	if time.Now().After(r.ExpiresAt) {
		_, _ = s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM idempotency_records WHERE id=?`), r.ID)
		return s.Begin(ctx, orgID, scope, key, requestHash, ttl)
	}
	if status.Valid {
		r.ResponseStatus = int(status.Int64)
	}
	r.ResponseBody = body
	return BeginResult{Conflict: r.RequestHash != requestHash, Record: r}, nil
}

func (s *Store) Complete(ctx context.Context, id string, status int, body []byte) error {
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE idempotency_records SET state='COMPLETED',response_status=?,response_body=? WHERE id=? AND state='PROCESSING'`), status, body, id)
	return err
}
func (s *Store) Forget(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM idempotency_records WHERE id=?`), id)
	return err
}
func (s *Store) PurgeExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM idempotency_records WHERE expires_at<?`), time.Now().UTC())
	return err
}
