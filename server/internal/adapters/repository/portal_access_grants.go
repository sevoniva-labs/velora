package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
	"github.com/sevoniva-labs/velora/server/internal/platform/reliablemsg"
)

type AccessImpactPreview struct {
	EffectiveUsers, AddedUsers, RevokedUsers, RoleChangedUsers, PrivilegedUsers, ProvisioningTasks int64
}

type accessProfile struct {
	portaldomain.AccessSubjectProfile
	Email, ExternalSubject, Status string
	ProvisioningVersion            int64
}

func (r *PortalRepo) ListAccessGrants(ctx context.Context, orgID, applicationID string) ([]portaldomain.AccessGrant, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,application_id,subject_type,subject_id,include_descendants,effect,valid_from,valid_until,status,reason,version,created_by,updated_by,created_at,updated_at FROM application_access_grants WHERE organization_id=? AND application_id=? ORDER BY created_at,id`), orgID, applicationID)
	if err != nil {
		return nil, err
	}
	items := make([]portaldomain.AccessGrant, 0)
	for rows.Next() {
		var item portaldomain.AccessGrant
		var validFrom, validUntil sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ApplicationID, &item.SubjectType, &item.SubjectID, &item.IncludeDescendants, &item.Effect, &validFrom, &validUntil, &item.Status, &item.Reason, &item.Version, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if validFrom.Valid {
			value := validFrom.Time
			item.ValidFrom = &value
		}
		if validUntil.Valid {
			value := validUntil.Time
			item.ValidUntil = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		roleRows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT role_key FROM application_access_grant_roles WHERE grant_id=? ORDER BY role_key`), items[i].ID)
		if err != nil {
			return nil, err
		}
		for roleRows.Next() {
			var role string
			if err := roleRows.Scan(&role); err != nil {
				_ = roleRows.Close()
				return nil, err
			}
			items[i].Roles = append(items[i].Roles, role)
		}
		_ = roleRows.Close()
		items[i].SubjectName = r.accessSubjectName(ctx, orgID, items[i])
	}
	return items, nil
}

func (r *PortalRepo) PreviewAccessGrants(ctx context.Context, orgID, applicationID string, grants []portaldomain.AccessGrant) (AccessImpactPreview, []portaldomain.EffectiveAccess, error) {
	profiles, parents, err := r.accessProfiles(ctx, orgID)
	if err != nil {
		return AccessImpactPreview{}, nil, err
	}
	current, err := r.currentEntitlements(ctx, applicationID)
	if err != nil {
		return AccessImpactPreview{}, nil, err
	}
	highRisk, err := r.highRiskApplicationRoles(ctx, orgID, applicationID)
	if err != nil {
		return AccessImpactPreview{}, nil, err
	}
	now := time.Now().UTC()
	preview := AccessImpactPreview{}
	effective := make([]portaldomain.EffectiveAccess, 0)
	for _, profile := range profiles {
		resolved := resolveProfileAccess(grants, profile, parents, now)
		before, existed := current[profile.UserID]
		if resolved.Allowed {
			preview.EffectiveUsers++
			effective = append(effective, resolved)
		}
		if resolved.Allowed && (!existed || !before.active) {
			preview.AddedUsers++
		}
		if !resolved.Allowed && existed && before.active {
			preview.RevokedUsers++
		}
		if resolved.Allowed && existed && before.active && !stringSlicesEqual(resolved.Roles, before.roles) {
			preview.RoleChangedUsers++
		}
		if resolved.Allowed && intersects(resolved.Roles, highRisk) {
			preview.PrivilegedUsers++
		}
	}
	preview.ProvisioningTasks = preview.AddedUsers + preview.RevokedUsers + preview.RoleChangedUsers
	return preview, effective, nil
}

func (r *PortalRepo) ReplaceAccessGrants(ctx context.Context, orgID, actorID, applicationID, issuer string, grants []portaldomain.AccessGrant) ([]portaldomain.AccessGrant, AccessImpactPreview, error) {
	if err := r.validateAccessGrants(ctx, orgID, applicationID, grants); err != nil {
		return nil, AccessImpactPreview{}, err
	}
	var preview AccessImpactPreview
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		preview, _, err = r.PreviewAccessGrants(ctx, orgID, applicationID, grants)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM application_access_grants WHERE organization_id=? AND application_id=?`), orgID, applicationID); err != nil {
			return err
		}
		now := time.Now().UTC()
		for i := range grants {
			grants[i].ID = uuid.NewString()
			grants[i].OrganizationID, grants[i].ApplicationID = orgID, applicationID
			grants[i].CreatedBy, grants[i].UpdatedBy = actorID, actorID
			grants[i].Version, grants[i].CreatedAt, grants[i].UpdatedAt = 1, now, now
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO application_access_grants(id,organization_id,application_id,subject_type,subject_id,include_descendants,effect,valid_from,valid_until,status,reason,version,created_by,updated_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), grants[i].ID, orgID, applicationID, grants[i].SubjectType, grants[i].SubjectID, grants[i].IncludeDescendants, grants[i].Effect, grants[i].ValidFrom, grants[i].ValidUntil, grants[i].Status, grants[i].Reason, 1, actorID, actorID, now, now); err != nil {
				return err
			}
			for _, role := range grants[i].Roles {
				if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO application_access_grant_roles(grant_id,role_key) VALUES(?,?)`), grants[i].ID, role); err != nil {
					return err
				}
			}
		}
		return r.recomputeAccessProjection(ctx, tx, orgID, actorID, applicationID, issuer, grants)
	})
	if err != nil {
		return nil, AccessImpactPreview{}, err
	}
	return grants, preview, nil
}

func (r *PortalRepo) ResolveApplicationAccess(ctx context.Context, orgID, applicationID, userID string) (portaldomain.EffectiveAccess, bool, error) {
	grants, err := r.ListAccessGrants(ctx, orgID, applicationID)
	if err != nil || len(grants) == 0 {
		return portaldomain.EffectiveAccess{}, false, err
	}
	profiles, parents, err := r.accessProfiles(ctx, orgID)
	if err != nil {
		return portaldomain.EffectiveAccess{}, true, err
	}
	for _, profile := range profiles {
		if profile.UserID == userID {
			return resolveProfileAccess(grants, profile, parents, time.Now().UTC()), true, nil
		}
	}
	return portaldomain.EffectiveAccess{UserID: userID}, true, nil
}

func (r *PortalRepo) ListUserEffectiveApplicationAccess(ctx context.Context, orgID, userID string) ([]portaldomain.UserEffectiveApplicationAccess, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT a.id,a.code,a.name,e.roles_json,e.status FROM user_application_entitlements e JOIN portal_applications a ON a.id=e.application_id AND a.organization_id=? JOIN users u ON u.id=e.user_id AND u.organization_id=a.organization_id WHERE e.user_id=? AND e.status='ACTIVE' AND u.status='ACTIVE' ORDER BY a.name,a.code`), orgID, userID)
	if err != nil {
		return nil, err
	}
	items := make([]portaldomain.UserEffectiveApplicationAccess, 0)
	for rows.Next() {
		var item portaldomain.UserEffectiveApplicationAccess
		var rolesJSON string
		if err := rows.Scan(&item.ApplicationID, &item.ApplicationCode, &item.ApplicationName, &rolesJSON, &item.Status); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.UserID = userID
		if err := json.Unmarshal([]byte(rolesJSON), &item.Roles); err != nil {
			_ = rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range items {
		grantRows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT g.id,g.subject_type,g.subject_id,g.effect FROM user_application_entitlement_sources s JOIN application_access_grants g ON g.id=s.grant_id AND g.organization_id=? WHERE s.user_id=? AND s.application_id=? ORDER BY g.subject_type,g.subject_id,g.id`), orgID, userID, items[index].ApplicationID)
		if err != nil {
			return nil, err
		}
		sources := make([]portaldomain.EffectiveApplicationAccessSource, 0)
		for grantRows.Next() {
			var source portaldomain.EffectiveApplicationAccessSource
			if err := grantRows.Scan(&source.GrantID, &source.SubjectType, &source.SubjectID, &source.Effect); err != nil {
				_ = grantRows.Close()
				return nil, err
			}
			sources = append(sources, source)
		}
		if err := grantRows.Close(); err != nil {
			return nil, err
		}
		for sourceIndex := range sources {
			sources[sourceIndex].SubjectName = r.accessSubjectName(ctx, orgID, portaldomain.AccessGrant{SubjectType: sources[sourceIndex].SubjectType, SubjectID: sources[sourceIndex].SubjectID})
		}
		items[index].Sources = sources
	}
	return items, nil
}

func (r *PortalRepo) validateAccessGrants(ctx context.Context, orgID, applicationID string, grants []portaldomain.AccessGrant) error {
	if len(grants) > 500 {
		return errors.New("too many application access grants")
	}
	var appCount int
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM portal_applications WHERE organization_id=? AND id=?`), orgID, applicationID).Scan(&appCount); err != nil || appCount != 1 {
		if err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	roleCatalog := make(map[string]struct{})
	roleRows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT role_key FROM portal_application_roles WHERE organization_id=? AND application_id=? AND status='ACTIVE'`), orgID, applicationID)
	if err != nil {
		return err
	}
	for roleRows.Next() {
		var key string
		if err := roleRows.Scan(&key); err != nil {
			_ = roleRows.Close()
			return err
		}
		roleCatalog[key] = struct{}{}
	}
	_ = roleRows.Close()
	seen := make(map[string]struct{}, len(grants))
	for i := range grants {
		grant := &grants[i]
		grant.SubjectType = strings.ToUpper(strings.TrimSpace(grant.SubjectType))
		grant.SubjectID = strings.TrimSpace(grant.SubjectID)
		grant.Effect = strings.ToUpper(strings.TrimSpace(grant.Effect))
		grant.Status = strings.ToUpper(strings.TrimSpace(grant.Status))
		grant.Reason = strings.TrimSpace(grant.Reason)
		if grant.Status == "" {
			grant.Status = portaldomain.StatusActive
		}
		if !validAccessSubject(grant.SubjectType) || grant.SubjectType != portaldomain.AccessSubjectEveryone && grant.SubjectID == "" || grant.Effect != portaldomain.AccessEffectAllow && grant.Effect != portaldomain.AccessEffectExclude || grant.Status != portaldomain.StatusActive && grant.Status != portaldomain.StatusDisabled || len(grant.Reason) > 500 || grant.ValidFrom != nil && grant.ValidUntil != nil && !grant.ValidUntil.After(*grant.ValidFrom) {
			return errors.New("invalid application access grant")
		}
		if grant.Effect == portaldomain.AccessEffectExclude && len(grant.Roles) > 0 {
			return errors.New("exclusion grant cannot contain roles")
		}
		grant.Roles = uniqueNonEmpty(grant.Roles)
		sort.Strings(grant.Roles)
		for _, role := range grant.Roles {
			if _, exists := roleCatalog[role]; !exists {
				return fmt.Errorf("application role %s is not active", role)
			}
		}
		key := grant.SubjectType + "\x00" + grant.SubjectID + "\x00" + grant.Effect
		if _, exists := seen[key]; exists {
			return errors.New("duplicate application access grant")
		}
		seen[key] = struct{}{}
		if err := r.validateAccessSubject(ctx, orgID, *grant); err != nil {
			return err
		}
	}
	return nil
}

func validAccessSubject(value string) bool {
	switch value {
	case portaldomain.AccessSubjectEveryone, portaldomain.AccessSubjectDepartment, portaldomain.AccessSubjectUserGroup, portaldomain.AccessSubjectPlatformRole, portaldomain.AccessSubjectUser:
		return true
	}
	return false
}

func (r *PortalRepo) validateAccessSubject(ctx context.Context, orgID string, grant portaldomain.AccessGrant) error {
	if grant.SubjectType == portaldomain.AccessSubjectEveryone {
		return nil
	}
	table, column := "", "id"
	switch grant.SubjectType {
	case portaldomain.AccessSubjectDepartment:
		table = "departments"
	case portaldomain.AccessSubjectUserGroup:
		table = "user_groups"
	case portaldomain.AccessSubjectPlatformRole:
		table, column = "roles", "role_key"
	case portaldomain.AccessSubjectUser:
		table = "users"
	}
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE organization_id=? AND %s=?", table, column)
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(query), orgID, grant.SubjectID).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return errors.New("application access subject does not exist")
	}
	return nil
}

func (r *PortalRepo) accessProfiles(ctx context.Context, orgID string) ([]accessProfile, map[string]string, error) {
	parents := make(map[string]string)
	departmentRows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,COALESCE(parent_id,'') FROM departments WHERE organization_id=? AND status='ACTIVE'`), orgID)
	if err != nil {
		return nil, nil, err
	}
	for departmentRows.Next() {
		var id, parent string
		if err := departmentRows.Scan(&id, &parent); err != nil {
			_ = departmentRows.Close()
			return nil, nil, err
		}
		parents[id] = parent
	}
	_ = departmentRows.Close()
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,login_name,display_name,email,external_subject,status,provisioning_version FROM users WHERE organization_id=? ORDER BY id`), orgID)
	if err != nil {
		return nil, nil, err
	}
	profiles := make([]accessProfile, 0)
	for rows.Next() {
		var profile accessProfile
		if err := rows.Scan(&profile.UserID, &profile.LoginName, &profile.DisplayName, &profile.Email, &profile.ExternalSubject, &profile.Status, &profile.ProvisioningVersion); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	for i := range profiles {
		profiles[i].Roles, err = r.stringColumn(ctx, `SELECT r.role_key FROM roles r JOIN user_roles ur ON ur.role_id=r.id WHERE r.organization_id=? AND ur.user_id=? ORDER BY r.role_key`, orgID, profiles[i].UserID)
		if err != nil {
			return nil, nil, err
		}
		profiles[i].GroupIDs, err = r.stringColumn(ctx, `SELECT ug.id FROM user_groups ug JOIN user_group_members ugm ON ugm.group_id=ug.id WHERE ug.organization_id=? AND ugm.user_id=? AND ug.status='ACTIVE' ORDER BY ug.id`, orgID, profiles[i].UserID)
		if err != nil {
			return nil, nil, err
		}
		profiles[i].DepartmentIDs, err = r.stringColumn(ctx, `SELECT DISTINCT department_id FROM user_assignments WHERE organization_id=? AND user_id=? AND valid_from<=? AND (valid_until IS NULL OR valid_until>?) ORDER BY department_id`, orgID, profiles[i].UserID, time.Now().UTC(), time.Now().UTC())
		if err != nil {
			return nil, nil, err
		}
	}
	return profiles, parents, nil
}

func (r *PortalRepo) stringColumn(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

type entitlementSnapshot struct {
	active bool
	roles  []string
}

func (r *PortalRepo) currentEntitlements(ctx context.Context, applicationID string) (map[string]entitlementSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT user_id,status,roles_json FROM user_application_entitlements WHERE application_id=?`), applicationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make(map[string]entitlementSnapshot)
	for rows.Next() {
		var userID, status, raw string
		if err := rows.Scan(&userID, &status, &raw); err != nil {
			return nil, err
		}
		var roles []string
		if err := json.Unmarshal([]byte(raw), &roles); err != nil {
			return nil, err
		}
		sort.Strings(roles)
		items[userID] = entitlementSnapshot{active: status == portaldomain.StatusActive, roles: roles}
	}
	return items, rows.Err()
}

func (r *PortalRepo) highRiskApplicationRoles(ctx context.Context, orgID, applicationID string) (map[string]struct{}, error) {
	values, err := r.stringColumn(ctx, `SELECT role_key FROM portal_application_roles WHERE organization_id=? AND application_id=? AND status='ACTIVE' AND risk_level IN ('PRIVILEGED','CRITICAL')`, orgID, applicationID)
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set, err
}

func (r *PortalRepo) recomputeAccessProjection(ctx context.Context, tx *sql.Tx, orgID, actorID, applicationID, issuer string, grants []portaldomain.AccessGrant) error {
	profiles, parents, err := r.accessProfiles(ctx, orgID)
	if err != nil {
		return err
	}
	current, err := r.currentEntitlements(ctx, applicationID)
	if err != nil {
		return err
	}
	var applicationCode string
	if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT code FROM portal_applications WHERE organization_id=? AND id=?`), orgID, applicationID).Scan(&applicationCode); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_application_entitlement_sources WHERE application_id=?`), applicationID); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, profile := range profiles {
		resolved := resolveProfileAccess(grants, profile, parents, now)
		before, existed := current[profile.UserID]
		if !resolved.Allowed && !existed {
			continue
		}
		status := portaldomain.StatusDisabled
		if resolved.Allowed {
			status = portaldomain.StatusActive
		}
		if existed && before.active == resolved.Allowed && stringSlicesEqual(before.roles, resolved.Roles) {
			for _, grantID := range resolved.SourceGrantIDs {
				if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_application_entitlement_sources(user_id,application_id,grant_id,created_at) VALUES(?,?,?,?)`), profile.UserID, applicationID, grantID, now); err != nil {
					return err
				}
			}
			continue
		}
		version := profile.ProvisioningVersion + 1
		raw, _ := json.Marshal(resolved.Roles)
		if r.db.Provider == "postgres" {
			_, err = tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_application_entitlements(user_id,application_id,roles_json,status,version,updated_by) VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,application_id) DO UPDATE SET roles_json=excluded.roles_json,status=excluded.status,version=excluded.version,updated_by=excluded.updated_by,updated_at=?`), profile.UserID, applicationID, string(raw), status, version, actorID, now)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO user_application_entitlements(user_id,application_id,roles_json,status,version,updated_by) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE roles_json=VALUES(roles_json),status=VALUES(status),version=VALUES(version),updated_by=VALUES(updated_by),updated_at=?`, profile.UserID, applicationID, string(raw), status, version, actorID, now)
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET provisioning_version=?,updated_at=? WHERE id=?`), version, now, profile.UserID); err != nil {
			return err
		}
		for _, grantID := range resolved.SourceGrantIDs {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_application_entitlement_sources(user_id,application_id,grant_id,created_at) VALUES(?,?,?,?)`), profile.UserID, applicationID, grantID, now); err != nil {
				return err
			}
		}
		if profile.ExternalSubject == "" {
			continue
		}
		event := EntitlementEvent{SchemaVersion: "1.0", EventID: uuid.NewString(), EventType: "user.entitlements.changed", AggregateVersion: version, OccurredAt: now, Source: "velora"}
		event.User.Subject, event.User.Issuer, event.User.LoginName, event.User.DisplayName, event.User.Email, event.User.Status = profile.ExternalSubject, issuer, profile.LoginName, profile.DisplayName, profile.Email, status
		event.Entitlements.ApplicationCode, event.Entitlements.Roles = applicationCode, resolved.Roles
		if _, err := reliablemsg.EnqueueTx(ctx, r.db, tx, reliablemsg.Event{ID: event.EventID, OrganizationID: orgID, Topic: "velora.provisioning." + applicationCode, Key: profile.UserID, Type: event.EventType, OrderingKey: profile.UserID, Payload: event}); err != nil {
			return err
		}
	}
	return nil
}

func resolveProfileAccess(grants []portaldomain.AccessGrant, profile accessProfile, parents map[string]string, now time.Time) portaldomain.EffectiveAccess {
	if profile.Status != portaldomain.StatusActive {
		return portaldomain.EffectiveAccess{UserID: profile.UserID, LoginName: profile.LoginName, DisplayName: profile.DisplayName}
	}
	return portaldomain.ResolveAccessGrants(grants, profile.AccessSubjectProfile, parents, now)
}

// RecomputeOrganizationAccess refreshes every v2 application projection in one
// transaction after organization, group, role, assignment, or user-state
// changes. Entitlement updates and reliable provisioning messages therefore
// commit atomically with the identity change that caused them.
func (r *PortalRepo) RecomputeOrganizationAccess(ctx context.Context, orgID, actorID, issuer string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, r.db.Rebind(`SELECT DISTINCT application_id FROM application_access_grants WHERE organization_id=? ORDER BY application_id`), orgID)
		if err != nil {
			return err
		}
		var applicationIDs []string
		for rows.Next() {
			var applicationID string
			if err := rows.Scan(&applicationID); err != nil {
				_ = rows.Close()
				return err
			}
			applicationIDs = append(applicationIDs, applicationID)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, applicationID := range applicationIDs {
			grants, err := r.ListAccessGrants(ctx, orgID, applicationID)
			if err != nil {
				return err
			}
			if err := r.recomputeAccessProjection(ctx, tx, orgID, actorID, applicationID, issuer, grants); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PortalRepo) accessSubjectName(ctx context.Context, orgID string, grant portaldomain.AccessGrant) string {
	if grant.SubjectType == portaldomain.AccessSubjectEveryone {
		return "全体成员"
	}
	table, column, display := "", "id", "name"
	switch grant.SubjectType {
	case portaldomain.AccessSubjectDepartment:
		table = "departments"
	case portaldomain.AccessSubjectUserGroup:
		table = "user_groups"
	case portaldomain.AccessSubjectPlatformRole:
		table, column = "roles", "role_key"
	case portaldomain.AccessSubjectUser:
		table, display = "users", "display_name"
	}
	var name string
	query := fmt.Sprintf("SELECT %s FROM %s WHERE organization_id=? AND %s=?", display, table, column)
	if r.db.QueryRowContext(ctx, r.db.Rebind(query), orgID, grant.SubjectID).Scan(&name) != nil {
		return ""
	}
	return name
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func intersects(values []string, set map[string]struct{}) bool {
	for _, value := range values {
		if _, exists := set[value]; exists {
			return true
		}
	}
	return false
}
