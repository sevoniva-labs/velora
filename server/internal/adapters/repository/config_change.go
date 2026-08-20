package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/sevoniva-labs/velora/server/internal/platform/configchange"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type ConfigChangeRepo struct{ db *database.DB }

func NewConfigChangeRepo(db *database.DB) *ConfigChangeRepo { return &ConfigChangeRepo{db: db} }

func (r *ConfigChangeRepo) List(ctx context.Context, organizationID string) ([]configchange.Change, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,namespace,config_group,data_id,version,expected_previous_version,value_digest,value_ref,sensitive,created_by,approved_by,approval_id,state,updated_at FROM config_change_history WHERE organization_id=? ORDER BY updated_at DESC,id DESC LIMIT 200`), organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]configchange.Change, 0)
	for rows.Next() {
		item, err := scanConfigChange(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ConfigChangeRepo) ByID(ctx context.Context, organizationID, id string) (configchange.Change, error) {
	return scanConfigChange(r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,namespace,config_group,data_id,version,expected_previous_version,value_digest,value_ref,sensitive,created_by,approved_by,approval_id,state,updated_at FROM config_change_history WHERE organization_id=? AND id=?`), organizationID, id))
}

func (r *ConfigChangeRepo) Create(ctx context.Context, change configchange.Change) (configchange.Change, error) {
	version, err := databaseVersion(change.Version)
	if err != nil {
		return configchange.Change{}, err
	}
	previousVersion, err := databaseVersion(change.ExpectedPreviousVersion)
	if err != nil {
		return configchange.Change{}, err
	}
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO config_change_history(id,organization_id,namespace,config_group,data_id,version,expected_previous_version,value_digest,value_ref,sensitive,created_by,approved_by,approval_id,state,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), change.ID, change.OrganizationID, change.Namespace, change.Group, change.DataID, version, previousVersion, change.ValueDigest, change.ValueRef, change.Sensitive, change.CreatedBy, nullIfEmpty(change.ApprovedBy), nullIfEmpty(change.ApprovalID), string(change.State), change.UpdatedAt)
	return change, err
}

func (r *ConfigChangeRepo) Update(ctx context.Context, change configchange.Change) (configchange.Change, error) {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE config_change_history SET approved_by=?,approval_id=?,state=?,updated_at=? WHERE organization_id=? AND id=?`), nullIfEmpty(change.ApprovedBy), nullIfEmpty(change.ApprovalID), string(change.State), change.UpdatedAt, change.OrganizationID, change.ID)
	if err != nil {
		return configchange.Change{}, err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return configchange.Change{}, err
		}
		return configchange.Change{}, sql.ErrNoRows
	}
	return change, nil
}

type configChangeScanner interface{ Scan(...any) error }

func scanConfigChange(scanner configChangeScanner) (configchange.Change, error) {
	var item configchange.Change
	var version, previous int64
	var approvedBy, approvalID sql.NullString
	var sensitive bool
	var state string
	if err := scanner.Scan(&item.ID, &item.OrganizationID, &item.Namespace, &item.Group, &item.DataID, &version, &previous, &item.ValueDigest, &item.ValueRef, &sensitive, &item.CreatedBy, &approvedBy, &approvalID, &state, &item.UpdatedAt); err != nil {
		return configchange.Change{}, err
	}
	if version < 0 || previous < 0 {
		return configchange.Change{}, errors.New("config version cannot be negative")
	}
	item.Version, item.ExpectedPreviousVersion, item.Sensitive, item.State = uint64(version), uint64(previous), sensitive, configchange.State(state)
	if approvedBy.Valid {
		item.ApprovedBy = approvedBy.String
	}
	if approvalID.Valid {
		item.ApprovalID = approvalID.String
	}
	if err := configchange.Validate(item); err != nil {
		return configchange.Change{}, fmt.Errorf("validate config change record: %w", err)
	}
	return item, nil
}

func databaseVersion(version uint64) (int64, error) {
	if version > math.MaxInt64 {
		return 0, errors.New("config version exceeds database integer range")
	}
	return int64(version), nil // #nosec G115 -- the upper bound is checked immediately above.
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
