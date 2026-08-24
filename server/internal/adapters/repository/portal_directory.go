package repository

import (
	"context"
	"encoding/json"
	"time"

	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

func (r *PortalRepo) GetDirectoryCredential(ctx context.Context, applicationID string) (portaldomain.DirectoryCredential, error) {
	var item portaldomain.DirectoryCredential
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT a.id,a.organization_id,a.code,a.lifecycle_status,t.secret_ref
		FROM portal_applications a
		JOIN portal_application_provisioning_targets t ON t.application_id=a.id AND t.organization_id=a.organization_id
		WHERE a.id=?`), applicationID).Scan(&item.ApplicationID, &item.OrganizationID, &item.ApplicationCode, &item.LifecycleStatus, &item.SecretRef)
	return item, err
}

func (r *PortalRepo) GetDirectoryOrganization(ctx context.Context, organizationID string) (portaldomain.DirectoryOrganization, error) {
	var item portaldomain.DirectoryOrganization
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,org_key,name,status,updated_at FROM organizations WHERE id=?`), organizationID).
		Scan(&item.ID, &item.Key, &item.Name, &item.Status, &item.UpdatedAt)
	return item, err
}

func (r *PortalRepo) ListDirectoryDepartments(ctx context.Context, organizationID string) ([]portaldomain.DirectoryDepartment, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,COALESCE(parent_id,''),department_key,name,status,sort_order,updated_at
		FROM departments WHERE organization_id=? ORDER BY sort_order,name,id`), organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]portaldomain.DirectoryDepartment, 0)
	for rows.Next() {
		var item portaldomain.DirectoryDepartment
		if err := rows.Scan(&item.ID, &item.ParentID, &item.Key, &item.Name, &item.Status, &item.SortOrder, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PortalRepo) ListDirectoryUsers(ctx context.Context, organizationID, applicationID string, updatedAfter, snapshotAt, cursorTime time.Time, cursorID string, limit int) ([]portaldomain.DirectoryUser, error) {
	query := `SELECT u.id,u.external_subject,u.login_name,u.display_name,u.email,COALESCE(u.department_id,''),
		CASE WHEN u.status='ACTIVE' AND e.status='ACTIVE' THEN 'ACTIVE' ELSE 'DISABLED' END,e.roles_json,e.version,
		CASE WHEN u.updated_at>e.updated_at THEN u.updated_at ELSE e.updated_at END
		FROM user_application_entitlements e
		JOIN users u ON u.id=e.user_id AND u.organization_id=?
		WHERE e.application_id=? AND u.external_subject<>''
		AND (CASE WHEN u.updated_at>e.updated_at THEN u.updated_at ELSE e.updated_at END)>?
		AND (CASE WHEN u.updated_at>e.updated_at THEN u.updated_at ELSE e.updated_at END)<=?
		AND ((CASE WHEN u.updated_at>e.updated_at THEN u.updated_at ELSE e.updated_at END)>?
			OR ((CASE WHEN u.updated_at>e.updated_at THEN u.updated_at ELSE e.updated_at END)=? AND u.id>?))
		ORDER BY (CASE WHEN u.updated_at>e.updated_at THEN u.updated_at ELSE e.updated_at END),u.id LIMIT ?`
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), organizationID, applicationID, updatedAfter, snapshotAt, cursorTime, cursorTime, cursorID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]portaldomain.DirectoryUser, 0)
	for rows.Next() {
		var item portaldomain.DirectoryUser
		var rolesJSON string
		if err := rows.Scan(&item.CursorID, &item.Subject, &item.LoginName, &item.DisplayName, &item.Email, &item.DepartmentID, &item.Status, &rolesJSON, &item.Version, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rolesJSON), &item.Roles); err != nil {
			return nil, err
		}
		if item.Status != portaldomain.StatusActive {
			item.Roles = nil
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
