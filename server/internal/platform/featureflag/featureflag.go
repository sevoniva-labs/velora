package featureflag

import (
	"context"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"time"
)

type Store struct{ db *database.DB }

func New(db *database.DB) *Store { return &Store{db: db} }
func (s *Store) Enabled(ctx context.Context, org, key string, defaults map[string]bool) bool {
	var v bool
	err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT enabled FROM feature_flags WHERE organization_id=? AND flag_key=?`), org, key).Scan(&v)
	if err == nil {
		return v
	}
	return defaults[key]
}
func (s *Store) Set(ctx context.Context, org, key string, enabled bool) error {
	var n int
	_ = s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT COUNT(*) FROM feature_flags WHERE organization_id=? AND flag_key=?`), org, key).Scan(&n)
	if n > 0 {
		_, err := s.db.ExecContext(ctx, s.db.Rebind(`UPDATE feature_flags SET enabled=?,updated_at=? WHERE organization_id=? AND flag_key=?`), enabled, time.Now().UTC(), org, key)
		return err
	}
	_, err := s.db.ExecContext(ctx, s.db.Rebind(`INSERT INTO feature_flags(organization_id,flag_key,enabled,updated_at) VALUES(?,?,?,?)`), org, key, enabled, time.Now().UTC())
	return err
}
