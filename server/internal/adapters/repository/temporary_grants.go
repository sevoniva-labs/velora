package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	identity "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func (r *IdentityRepo) CreateTemporaryRoleGrant(ctx context.Context, grant identity.TemporaryRoleGrant) (identity.TemporaryRoleGrant, error) {
	var roleID string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT r.id FROM roles r JOIN users u ON u.organization_id=r.organization_id WHERE u.id=? AND u.organization_id=? AND r.role_key=? AND r.status='ACTIVE'`), grant.UserID, grant.OrganizationID, grant.RoleKey).Scan(&roleID)
	if err != nil {
		return identity.TemporaryRoleGrant{}, err
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_role_exclusions WHERE organization_id=? AND user_id=? AND role_id=?`), grant.OrganizationID, grant.UserID, roleID); err != nil {
		return identity.TemporaryRoleGrant{}, err
	}
	grant.ID = uuid.NewString()
	grant.CreatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO temporary_role_grants(id,organization_id,user_id,role_id,requested_by,approval_id,reason,valid_from,valid_until,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`),
		grant.ID, grant.OrganizationID, grant.UserID, roleID, grant.RequestedBy, grant.ApprovalID, grant.Reason, grant.ValidFrom, grant.ValidUntil, grant.CreatedAt)
	return grant, err
}

func (r *IdentityRepo) ListTemporaryRoleGrants(ctx context.Context, organizationID string) ([]identity.TemporaryRoleGrant, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT g.id,g.organization_id,g.user_id,r.role_key,g.requested_by,g.approval_id,g.reason,g.valid_from,g.valid_until,g.revoked_at,g.revoked_by,g.revoke_reason,g.created_at FROM temporary_role_grants g JOIN roles r ON r.id=g.role_id WHERE g.organization_id=? ORDER BY g.created_at DESC LIMIT 1000`), organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	grants := make([]identity.TemporaryRoleGrant, 0)
	for rows.Next() {
		var grant identity.TemporaryRoleGrant
		var revokedAt sql.NullTime
		var revokedBy, revokeReason sql.NullString
		if err := rows.Scan(&grant.ID, &grant.OrganizationID, &grant.UserID, &grant.RoleKey, &grant.RequestedBy, &grant.ApprovalID, &grant.Reason, &grant.ValidFrom, &grant.ValidUntil, &revokedAt, &revokedBy, &revokeReason, &grant.CreatedAt); err != nil {
			return nil, err
		}
		if revokedAt.Valid {
			value := revokedAt.Time
			grant.RevokedAt = &value
		}
		grant.RevokedBy = revokedBy.String
		grant.RevokeReason = revokeReason.String
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (r *IdentityRepo) RevokeTemporaryRoleGrant(ctx context.Context, organizationID, grantID, actorID, reason string) error {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE temporary_role_grants SET revoked_at=?,revoked_by=?,revoke_reason=? WHERE id=? AND organization_id=? AND revoked_at IS NULL`), time.Now().UTC(), actorID, reason, grantID, organizationID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
