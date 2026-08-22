package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type IdentityRepo struct{ db *database.DB }

var ErrLastSystemAdmin = errors.New("cannot remove or disable the last active system administrator")
var ErrPasswordStateChanged = errors.New("password state changed concurrently")
var ErrMFAAlreadyEnabled = errors.New("multi-factor authentication is already enabled")

func NewIdentityRepo(db *database.DB) *IdentityRepo { return &IdentityRepo{db: db} }

func (r *IdentityRepo) dbProvider() string { return r.db.Provider }

type userRow struct {
	User         identity.User
	PasswordHash string
}

func (r *IdentityRepo) EnsureOrganization(ctx context.Context, key, name string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO organizations(id,org_key,name,status,description,max_users,max_active_sessions,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`), id, key, name, "ACTIVE", "", 0, 0, now, now)
	if err == nil {
		return id, nil
	}
	// Another instance may have inserted the same bootstrap record between the
	// SELECT and INSERT. Re-read by the natural key so startup remains
	// idempotent across rolling deployments and concurrent Pod starts.
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), key).Scan(&existing); readErr == nil {
		return existing, nil
	}
	return "", err
}
func (r *IdentityRepo) EnsureRole(ctx context.Context, orgID, key, name, dataScope string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO roles(id,organization_id,role_key,name,data_scope_type,created_at) VALUES(?,?,?,?,?,?)`), id, orgID, key, name, dataScope, time.Now().UTC())
	if err == nil {
		return id, nil
	}
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, key).Scan(&existing); readErr == nil {
		return existing, nil
	}
	return "", err
}

func (r *IdentityRepo) EnsurePermission(ctx context.Context, key, name string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), key).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id = uuid.NewString()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO permissions(id,permission_key,name,created_at) VALUES(?,?,?,?)`), id, key, name, time.Now().UTC())
	if err == nil {
		return id, nil
	}
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), key).Scan(&existing); readErr == nil {
		return existing, nil
	}
	return "", err
}
func (r *IdentityRepo) GrantPermissionToRole(ctx context.Context, orgID, roleKey, permissionKey string) error {
	var roleID, permissionID string
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
		return err
	}
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), permissionKey).Scan(&permissionID); err != nil {
		return err
	}
	var n int
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM role_permissions WHERE role_id=? AND permission_id=?`), roleID, permissionID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO role_permissions(role_id,permission_id) VALUES(?,?)`), roleID, permissionID)
	if err == nil {
		return nil
	}
	// Concurrent seeders can race on the composite primary key. Treat the
	// operation as successful if the relationship now exists.
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM role_permissions WHERE role_id=? AND permission_id=?`), roleID, permissionID).Scan(&n); readErr == nil && n > 0 {
		return nil
	}
	return err
}

func (r *IdentityRepo) UserByLogin(ctx context.Context, orgID, login string) (userRow, error) {
	var out userRow
	var locked sql.NullTime
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,login_name,display_name,email,identity_source,external_subject,provisioning_version,password_hash,status,must_change_password,failed_login_count,locked_until,password_changed_at,created_at,updated_at FROM users WHERE organization_id=? AND login_name=?`), orgID, login).
		Scan(&out.User.ID, &out.User.OrganizationID, &out.User.LoginName, &out.User.DisplayName, &out.User.Email, &out.User.IdentitySource, &out.User.ExternalSubject, &out.User.ProvisioningVersion, &out.PasswordHash, &out.User.Status, &out.User.MustChangePassword, &out.User.FailedLoginCount, &locked, &out.User.PasswordChangedAt, &out.User.CreatedAt, &out.User.UpdatedAt)
	if locked.Valid {
		t := locked.Time
		out.User.LockedUntil = &t
	}
	if err == nil {
		out.User.Roles, _ = r.RolesForUser(ctx, out.User.ID)
		out.User.Permissions, _ = r.PermissionsForUser(ctx, out.User.ID)
		out.User.Entitlements, _ = r.ListUserEntitlements(ctx, out.User.ID)
	}
	return out, err
}
func (r *IdentityRepo) UserByID(ctx context.Context, id string) (identity.User, error) {
	var out identity.User
	var locked sql.NullTime
	var hash string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,login_name,display_name,email,identity_source,external_subject,provisioning_version,password_hash,status,must_change_password,failed_login_count,locked_until,password_changed_at,created_at,updated_at FROM users WHERE id=?`), id).
		Scan(&out.ID, &out.OrganizationID, &out.LoginName, &out.DisplayName, &out.Email, &out.IdentitySource, &out.ExternalSubject, &out.ProvisioningVersion, &hash, &out.Status, &out.MustChangePassword, &out.FailedLoginCount, &locked, &out.PasswordChangedAt, &out.CreatedAt, &out.UpdatedAt)
	if locked.Valid {
		t := locked.Time
		out.LockedUntil = &t
	}
	if err == nil {
		out.Roles, _ = r.RolesForUser(ctx, id)
		out.Permissions, _ = r.PermissionsForUser(ctx, id)
		out.Entitlements, _ = r.ListUserEntitlements(ctx, id)
	}
	return out, err
}
func (r *IdentityRepo) CreateUser(ctx context.Context, orgID, login, display, passwordHash string, mustChange bool) (identity.User, error) {
	now := time.Now().UTC()
	u := identity.User{ID: uuid.NewString(), OrganizationID: orgID, LoginName: login, DisplayName: display, Status: "ACTIVE", MustChangePassword: mustChange, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO users(id,organization_id,login_name,display_name,password_hash,status,must_change_password,failed_login_count,password_changed_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`), u.ID, orgID, login, display, passwordHash, u.Status, mustChange, 0, now, now, now)
	return u, err
}
func (r *IdentityRepo) CreateUserWithRoles(ctx context.Context, orgID, login, display, passwordHash string, mustChange bool, roles []string) (identity.User, error) {
	now := time.Now().UTC()
	u := identity.User{ID: uuid.NewString(), OrganizationID: orgID, LoginName: login, DisplayName: display, Status: "ACTIVE", MustChangePassword: mustChange, PasswordChangedAt: now, CreatedAt: now, UpdatedAt: now, Roles: roles}
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO users(id,organization_id,login_name,display_name,password_hash,status,must_change_password,failed_login_count,password_changed_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`), u.ID, orgID, login, display, passwordHash, u.Status, mustChange, 0, now, now, now); err != nil {
			return err
		}
		for _, roleKey := range roles {
			var roleID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_roles(user_id,role_id) VALUES(?,?)`), u.ID, roleID); err != nil {
				return err
			}
		}
		return nil
	})
	return u, err
}
func (r *IdentityRepo) GrantRole(ctx context.Context, userID, roleKey string) error {
	var roleID string
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT r.id FROM roles r JOIN users u ON u.organization_id=r.organization_id WHERE u.id=? AND r.role_key=?`), userID, roleKey).Scan(&roleID); err != nil {
		return err
	}
	var n int
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles WHERE user_id=? AND role_id=?`), userID, roleID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_roles(user_id,role_id) VALUES(?,?)`), userID, roleID)
	if err == nil {
		return nil
	}
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles WHERE user_id=? AND role_id=?`), userID, roleID).Scan(&n); readErr == nil && n > 0 {
		return nil
	}
	return err
}
func (r *IdentityRepo) RolesForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT role_key FROM (
		SELECT r.role_key AS role_key FROM roles r JOIN user_roles ur ON ur.role_id=r.id WHERE ur.user_id=?
		UNION
		SELECT r.role_key AS role_key FROM roles r
		JOIN user_group_roles ugr ON ugr.role_id=r.id
		JOIN user_groups ug ON ug.id=ugr.group_id AND ug.status='ACTIVE'
		JOIN user_group_members ugm ON ugm.group_id=ug.id
		WHERE ugm.user_id=?
		UNION
		SELECT r.role_key AS role_key FROM roles r JOIN temporary_role_grants trg ON trg.role_id=r.id
		WHERE trg.user_id=? AND trg.revoked_at IS NULL AND trg.valid_from<=? AND trg.valid_until>?
	) effective_roles ORDER BY role_key`), userID, userID, userID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *IdentityRepo) PermissionsForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT DISTINCT p.permission_key FROM permissions p
		JOIN role_permissions rp ON rp.permission_id=p.id
		JOIN (
			SELECT role_id FROM user_roles WHERE user_id=?
			UNION
			SELECT ugr.role_id FROM user_group_roles ugr
			JOIN user_groups ug ON ug.id=ugr.group_id AND ug.status='ACTIVE'
			JOIN user_group_members ugm ON ugm.group_id=ug.id
			WHERE ugm.user_id=?
			UNION
			SELECT role_id FROM temporary_role_grants WHERE user_id=? AND revoked_at IS NULL AND valid_from<=? AND valid_until>?
		) effective_roles ON effective_roles.role_id=rp.role_id
		ORDER BY p.permission_key`), userID, userID, userID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var x string
		if err := rows.Scan(&x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *IdentityRepo) RecordLoginFailure(ctx context.Context, userID string, max int, lock time.Duration) error {
	if max <= 0 {
		max = 1
	}
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		// Row lock prevents lost increments when multiple API replicas receive
		// simultaneous failed login attempts for the same account. FOR UPDATE is
		// supported by PostgreSQL, MySQL/InnoDB and OceanBase MySQL mode.
		var failures int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT failed_login_count FROM users WHERE id=? FOR UPDATE`), userID).Scan(&failures); err != nil {
			return err
		}
		failures++
		now := time.Now().UTC()
		var until any
		if failures >= max {
			until = now.Add(lock)
			failures = 0
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET failed_login_count=?,locked_until=?,updated_at=? WHERE id=?`), failures, until, now, userID)
		return err
	})
}
func (r *IdentityRepo) ResetLoginFailure(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE users SET failed_login_count=0,locked_until=NULL,updated_at=? WHERE id=?`), time.Now().UTC(), userID)
	return err
}
func (r *IdentityRepo) CreateSession(ctx context.Context, userID, tokenHash string, expires time.Time, ip, ua, authenticationLevel string, mfaVerifiedAt *time.Time) (string, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO sessions(id,user_id,token_hash,expires_at,created_at,last_seen_at,client_ip,user_agent,authentication_level,mfa_verified_at) VALUES(?,?,?,?,?,?,?,?,?,?)`), id, userID, tokenHash, expires, now, now, ip, ua, authenticationLevel, mfaVerifiedAt)
	return id, err
}
func (r *IdentityRepo) PrincipalBySessionHash(ctx context.Context, hash string) (identity.Principal, error) {
	return r.principalBySession(ctx, `s.token_hash=?`, hash)
}

// PrincipalBySessionID revalidates a server-side session reference without
// requiring the raw bearer token. It is used by companion authentication
// sessions that must remain linked to the originating Velora session.
func (r *IdentityRepo) PrincipalBySessionID(ctx context.Context, sessionID string) (identity.Principal, error) {
	return r.principalBySession(ctx, `s.id=?`, strings.TrimSpace(sessionID))
}

func (r *IdentityRepo) principalBySession(ctx context.Context, predicate, value string) (identity.Principal, error) {
	var p identity.Principal
	if strings.TrimSpace(value) == "" {
		return p, sql.ErrNoRows
	}
	var exp time.Time
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT s.id,u.id,u.organization_id,u.login_name,u.display_name,u.must_change_password,u.password_changed_at,s.expires_at FROM sessions s JOIN users u ON u.id=s.user_id WHERE `+predicate+` AND u.status='ACTIVE'`), value).
		Scan(&p.SessionID, &p.UserID, &p.OrganizationID, &p.LoginName, &p.DisplayName, &p.MustChangePassword, &p.PasswordChangedAt, &exp)
	if err != nil {
		return p, err
	}
	p.Type = "USER"
	var mfaVerified sql.NullTime
	if err = r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT authentication_level,mfa_verified_at FROM sessions WHERE id=?`), p.SessionID).Scan(&p.AuthenticationLevel, &mfaVerified); err != nil {
		return p, err
	}
	if mfaVerified.Valid {
		value := mfaVerified.Time
		p.MFAVerifiedAt = &value
	}
	if time.Now().After(exp) {
		_ = r.DeleteSessionByID(ctx, p.SessionID)
		return p, sql.ErrNoRows
	}
	p.Roles, _ = r.RolesForUser(ctx, p.UserID)
	p.Permissions, _ = r.PermissionsForUser(ctx, p.UserID)
	p.DataScope, err = r.DataScopeForUser(ctx, p.OrganizationID, p.UserID)
	if err != nil {
		return p, err
	}
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE sessions SET last_seen_at=? WHERE id=?`), time.Now().UTC(), p.SessionID)
	return p, nil
}

func (r *IdentityRepo) MarkSessionMFAVerified(ctx context.Context, userID, sessionID string, verifiedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE sessions SET authentication_level='MFA',mfa_verified_at=?,last_seen_at=? WHERE id=? AND user_id=? AND expires_at>?`), verifiedAt, verifiedAt, sessionID, userID, verifiedAt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return sql.ErrNoRows
	}
	return nil
}
func (r *IdentityRepo) DeleteSessionByHash(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE token_hash=?`), hash)
	return err
}
func (r *IdentityRepo) DeleteSessionByID(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE id=?`), id)
	return err
}
func (r *IdentityRepo) PurgeExpiredSessions(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE expires_at<?`), time.Now().UTC())
	return err
}

func (r *IdentityRepo) PasswordHashByID(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT password_hash FROM users WHERE id=?`), userID).Scan(&hash)
	return hash, err
}
func (r *IdentityRepo) PasswordHistory(ctx context.Context, userID string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT password_hash FROM password_history WHERE user_id=? ORDER BY changed_at DESC LIMIT ?`), userID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (r *IdentityRepo) UpdatePasswordAndRevokeOtherSessions(ctx context.Context, userID, keepSessionID, oldHash, newHash string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO password_history(id,user_id,password_hash,changed_at) VALUES(?,?,?,?)`), uuid.NewString(), userID, oldHash, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET password_hash=?,must_change_password=false,failed_login_count=0,locked_until=NULL,password_changed_at=?,updated_at=? WHERE id=?`), newHash, now, now, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=? AND id<>?`), userID, keepSessionID)
		return err
	})
}
func (r *IdentityRepo) ListUsers(ctx context.Context, orgID, actorUserID string, scope identity.EffectiveDataScope, limit int) ([]identity.User, error) {
	query := `SELECT u.id,u.organization_id,u.login_name,u.display_name,u.email,u.identity_source,u.external_subject,u.provisioning_version,u.status,u.must_change_password,u.locked_until,u.password_changed_at,u.created_at,u.updated_at FROM users u WHERE u.organization_id=?`
	args := []any{orgID}
	if !scope.OrganizationWide {
		conditions := make([]string, 0, 2)
		if scope.Self && actorUserID != "" {
			conditions = append(conditions, `u.id=?`)
			args = append(args, actorUserID)
		}
		if len(scope.DepartmentIDs) > 0 {
			placeholders := make([]string, len(scope.DepartmentIDs))
			for index, departmentID := range scope.DepartmentIDs {
				placeholders[index] = "?"
				args = append(args, departmentID)
			}
			now := time.Now().UTC()
			conditions = append(conditions, `EXISTS (SELECT 1 FROM user_assignments ua WHERE ua.organization_id=u.organization_id AND ua.user_id=u.id AND ua.department_id IN (`+strings.Join(placeholders, ",")+`) AND ua.valid_from<=? AND (ua.valid_until IS NULL OR ua.valid_until>?))`)
			args = append(args, now, now)
		}
		if len(conditions) == 0 {
			return []identity.User{}, nil
		}
		query += ` AND (` + strings.Join(conditions, ` OR `) + `)`
	}
	query += ` ORDER BY u.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []identity.User
	for rows.Next() {
		var u identity.User
		var locked sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.LoginName, &u.DisplayName, &u.Email, &u.IdentitySource, &u.ExternalSubject, &u.ProvisioningVersion, &u.Status, &u.MustChangePassword, &locked, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if locked.Valid {
			x := locked.Time
			u.LockedUntil = &x
		}
		u.Roles, _ = r.RolesForUser(ctx, u.ID)
		u.Permissions, _ = r.PermissionsForUser(ctx, u.ID)
		u.Entitlements, _ = r.ListUserEntitlements(ctx, u.ID)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) CreateAPIToken(ctx context.Context, userID, name, prefix, tokenHash string, scopes []string, expiresAt *time.Time) (identity.APIToken, error) {
	user, err := r.UserByID(ctx, userID)
	if err != nil {
		return identity.APIToken{}, err
	}
	raw, _ := json.Marshal(scopes)
	now := time.Now().UTC()
	t := identity.APIToken{ID: uuid.NewString(), Name: name, Prefix: prefix, Scopes: scopes, ExpiresAt: expiresAt, CreatedAt: now}
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO api_tokens(id,organization_id,user_id,name,token_prefix,token_hash,scopes_json,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?)`), t.ID, user.OrganizationID, userID, name, prefix, tokenHash, string(raw), expiresAt, now)
	return t, err
}

func (r *IdentityRepo) ListAPITokens(ctx context.Context, userID string) ([]identity.APIToken, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,name,token_prefix,scopes_json,expires_at,last_used_at,created_at FROM api_tokens WHERE user_id=? AND revoked_at IS NULL ORDER BY created_at DESC LIMIT 200`), userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []identity.APIToken
	for rows.Next() {
		var t identity.APIToken
		var scopesRaw string
		var expires, last sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Prefix, &scopesRaw, &expires, &last, &t.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(scopesRaw), &t.Scopes)
		if expires.Valid {
			x := expires.Time
			t.ExpiresAt = &x
		}
		if last.Valid {
			x := last.Time
			t.LastUsedAt = &x
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) RevokeAPIToken(ctx context.Context, userID, tokenID string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE api_tokens SET revoked_at=? WHERE id=? AND user_id=? AND revoked_at IS NULL`), time.Now().UTC(), tokenID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) PrincipalByAPITokenHash(ctx context.Context, hash string) (identity.Principal, error) {
	var p identity.Principal
	var scopesRaw string
	var expires sql.NullTime
	var tokenID string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT t.id,u.id,u.organization_id,u.login_name,u.display_name,u.must_change_password,u.password_changed_at,t.scopes_json,t.expires_at FROM api_tokens t JOIN users u ON u.id=t.user_id WHERE t.token_hash=? AND t.revoked_at IS NULL AND u.status='ACTIVE'`), hash).
		Scan(&tokenID, &p.UserID, &p.OrganizationID, &p.LoginName, &p.DisplayName, &p.MustChangePassword, &p.PasswordChangedAt, &scopesRaw, &expires)
	if err != nil {
		return p, err
	}
	if expires.Valid && time.Now().After(expires.Time) {
		return p, sql.ErrNoRows
	}
	p.Type = "TOKEN"
	_ = json.Unmarshal([]byte(scopesRaw), &p.Scopes)
	p.Roles, _ = r.RolesForUser(ctx, p.UserID)
	p.Permissions, _ = r.PermissionsForUser(ctx, p.UserID)
	p.DataScope, err = r.DataScopeForUser(ctx, p.OrganizationID, p.UserID)
	if err != nil {
		return p, err
	}
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE api_tokens SET last_used_at=? WHERE id=?`), time.Now().UTC(), tokenID)
	return p, nil
}

func (r *IdentityRepo) OrganizationByID(ctx context.Context, orgID string) (identity.Organization, error) {
	var out identity.Organization
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,org_key,name,status,description,max_users,max_active_sessions,created_at,updated_at FROM organizations WHERE id=?`), orgID).
		Scan(&out.ID, &out.Key, &out.Name, &out.Status, &out.Description, &out.MaxUsers, &out.MaxSessions, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *IdentityRepo) UpdateOrganization(ctx context.Context, orgID string, req identity.Organization) (identity.Organization, error) {
	if req.Status != "ACTIVE" && req.Status != "DISABLED" {
		return identity.Organization{}, errors.New("invalid organization status")
	}
	if req.Name = strings.TrimSpace(req.Name); req.Name == "" {
		return identity.Organization{}, errors.New("invalid organization name")
	}
	if req.Description = strings.TrimSpace(req.Description); req.Description == "" {
		req.Description = ""
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE organizations SET name=?,description=?,status=?,max_users=?,max_active_sessions=?,updated_at=? WHERE id=?`), req.Name, req.Description, req.Status, req.MaxUsers, req.MaxSessions, time.Now().UTC(), orgID); err != nil {
		return identity.Organization{}, err
	}
	return r.OrganizationByID(ctx, orgID)
}

func (r *IdentityRepo) ListDepartments(ctx context.Context, orgID string) ([]identity.Department, error) {
	return r.listDepartments(ctx, orgID, false)
}

func (r *IdentityRepo) ListDepartmentsForUpdate(ctx context.Context, orgID string) ([]identity.Department, error) {
	return r.listDepartments(ctx, orgID, true)
}

func (r *IdentityRepo) listDepartments(ctx context.Context, orgID string, forUpdate bool) ([]identity.Department, error) {
	query := `SELECT id,organization_id,parent_id,department_key,name,status,sort_order,created_at,updated_at
		FROM departments WHERE organization_id=? ORDER BY sort_order,department_key`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]identity.Department, 0)
	for rows.Next() {
		var item identity.Department
		var parent sql.NullString
		if err := rows.Scan(&item.ID, &item.OrganizationID, &parent, &item.Key, &item.Name, &item.Status, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if parent.Valid {
			item.ParentID = parent.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) DepartmentByID(ctx context.Context, orgID, departmentID string) (identity.Department, error) {
	var out identity.Department
	var parent sql.NullString
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,parent_id,department_key,name,status,sort_order,created_at,updated_at FROM departments WHERE organization_id=? AND id=?`), orgID, departmentID).
		Scan(&out.ID, &out.OrganizationID, &parent, &out.Key, &out.Name, &out.Status, &out.SortOrder, &out.CreatedAt, &out.UpdatedAt)
	if parent.Valid {
		out.ParentID = parent.String
	}
	return out, err
}

func (r *IdentityRepo) CreateDepartment(ctx context.Context, req identity.Department) (identity.Department, error) {
	now := time.Now().UTC()
	req.ID = uuid.NewString()
	req.CreatedAt = now
	req.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO departments(id,organization_id,parent_id,department_key,name,status,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`),
		req.ID, req.OrganizationID, nullableString(req.ParentID), req.Key, req.Name, req.Status, req.SortOrder, now, now)
	if err != nil {
		return identity.Department{}, err
	}
	return req, nil
}

func (r *IdentityRepo) UpdateDepartment(ctx context.Context, req identity.Department) (identity.Department, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE departments SET parent_id=?,name=?,status=?,sort_order=?,updated_at=? WHERE organization_id=? AND id=?`),
		nullableString(req.ParentID), req.Name, req.Status, req.SortOrder, now, req.OrganizationID, req.ID)
	if err != nil {
		return identity.Department{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return identity.Department{}, err
	}
	if count != 1 {
		return identity.Department{}, sql.ErrNoRows
	}
	return r.DepartmentByID(ctx, req.OrganizationID, req.ID)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *IdentityRepo) ListPositions(ctx context.Context, orgID string) ([]identity.Position, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,department_id,position_key,name,description,status,sort_order,created_at,updated_at FROM positions WHERE organization_id=? ORDER BY sort_order,position_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]identity.Position, 0)
	for rows.Next() {
		var item identity.Position
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.DepartmentID, &item.Key, &item.Name, &item.Description, &item.Status, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) PositionByID(ctx context.Context, orgID, positionID string) (identity.Position, error) {
	var out identity.Position
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,department_id,position_key,name,description,status,sort_order,created_at,updated_at FROM positions WHERE organization_id=? AND id=?`), orgID, positionID).
		Scan(&out.ID, &out.OrganizationID, &out.DepartmentID, &out.Key, &out.Name, &out.Description, &out.Status, &out.SortOrder, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *IdentityRepo) CreatePosition(ctx context.Context, req identity.Position) (identity.Position, error) {
	now := time.Now().UTC()
	req.ID = uuid.NewString()
	req.CreatedAt = now
	req.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO positions(id,organization_id,department_id,position_key,name,description,status,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`),
		req.ID, req.OrganizationID, req.DepartmentID, req.Key, req.Name, req.Description, req.Status, req.SortOrder, now, now)
	if err != nil {
		return identity.Position{}, err
	}
	return req, nil
}

func (r *IdentityRepo) UpdatePosition(ctx context.Context, req identity.Position) (identity.Position, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE positions SET department_id=?,name=?,description=?,status=?,sort_order=?,updated_at=? WHERE organization_id=? AND id=?`),
		req.DepartmentID, req.Name, req.Description, req.Status, req.SortOrder, now, req.OrganizationID, req.ID)
	if err != nil {
		return identity.Position{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return identity.Position{}, err
	}
	if count != 1 {
		return identity.Position{}, sql.ErrNoRows
	}
	return r.PositionByID(ctx, req.OrganizationID, req.ID)
}

func (r *IdentityRepo) CountActivePositions(ctx context.Context, orgID, departmentID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM positions WHERE organization_id=? AND department_id=? AND status='ACTIVE'`), orgID, departmentID).Scan(&count)
	return count, err
}

func (r *IdentityRepo) CountCurrentAssignments(ctx context.Context, orgID, departmentID string, at time.Time) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_assignments WHERE organization_id=? AND department_id=? AND valid_from<=? AND (valid_until IS NULL OR valid_until>?)`), orgID, departmentID, at, at).Scan(&count)
	return count, err
}

func (r *IdentityRepo) ListUserGroups(ctx context.Context, orgID string) ([]identity.UserGroup, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,group_key,name,description,status,created_at,updated_at FROM user_groups WHERE organization_id=? ORDER BY group_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]identity.UserGroup, 0)
	for rows.Next() {
		var item identity.UserGroup
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Key, &item.Name, &item.Description, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range out {
		members, err := r.UserGroupMemberIDs(ctx, out[index].OrganizationID, out[index].ID)
		if err != nil {
			return nil, err
		}
		roles, err := r.UserGroupRoleKeys(ctx, out[index].OrganizationID, out[index].ID)
		if err != nil {
			return nil, err
		}
		out[index].MemberIDs = members
		out[index].MemberCount = len(members)
		out[index].Roles = roles
	}
	return out, nil
}

func (r *IdentityRepo) UserGroupByID(ctx context.Context, orgID, groupID string) (identity.UserGroup, error) {
	var out identity.UserGroup
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,group_key,name,description,status,created_at,updated_at FROM user_groups WHERE organization_id=? AND id=?`), orgID, groupID).
		Scan(&out.ID, &out.OrganizationID, &out.Key, &out.Name, &out.Description, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return out, err
	}
	out.MemberIDs, err = r.UserGroupMemberIDs(ctx, orgID, groupID)
	if err != nil {
		return identity.UserGroup{}, err
	}
	out.MemberCount = len(out.MemberIDs)
	out.Roles, err = r.UserGroupRoleKeys(ctx, orgID, groupID)
	return out, err
}

func (r *IdentityRepo) CreateUserGroup(ctx context.Context, req identity.UserGroup) (identity.UserGroup, error) {
	now := time.Now().UTC()
	req.ID = uuid.NewString()
	req.CreatedAt = now
	req.UpdatedAt = now
	req.Roles = []string{}
	req.MemberIDs = []string{}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_groups(id,organization_id,group_key,name,description,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`),
		req.ID, req.OrganizationID, req.Key, req.Name, req.Description, req.Status, now, now)
	if err != nil {
		return identity.UserGroup{}, err
	}
	return req, nil
}

func (r *IdentityRepo) UpdateUserGroup(ctx context.Context, req identity.UserGroup) (identity.UserGroup, error) {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE user_groups SET name=?,description=?,status=?,updated_at=? WHERE organization_id=? AND id=?`),
		req.Name, req.Description, req.Status, time.Now().UTC(), req.OrganizationID, req.ID)
	if err != nil {
		return identity.UserGroup{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return identity.UserGroup{}, err
	}
	if count != 1 {
		return identity.UserGroup{}, sql.ErrNoRows
	}
	return r.UserGroupByID(ctx, req.OrganizationID, req.ID)
}

func (r *IdentityRepo) UserGroupMemberIDs(ctx context.Context, orgID, groupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT ugm.user_id FROM user_group_members ugm JOIN user_groups ug ON ug.id=ugm.group_id WHERE ug.organization_id=? AND ug.id=? ORDER BY ugm.user_id`), orgID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) UserGroupRoleKeys(ctx context.Context, orgID, groupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT r.role_key FROM user_group_roles ugr JOIN user_groups ug ON ug.id=ugr.group_id JOIN roles r ON r.id=ugr.role_id WHERE ug.organization_id=? AND ug.id=? ORDER BY r.role_key`), orgID, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) ReplaceUserGroupMembers(ctx context.Context, orgID, groupID string, memberIDs []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var groupOrg string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id FROM user_groups WHERE id=? FOR UPDATE`), groupID).Scan(&groupOrg); err != nil {
			return err
		}
		if groupOrg != orgID {
			return sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_group_members WHERE group_id=?`), groupID); err != nil {
			return err
		}
		for _, userID := range memberIDs {
			var userOrg string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id FROM users WHERE id=?`), userID).Scan(&userOrg); err != nil {
				return err
			}
			if userOrg != orgID {
				return sql.ErrNoRows
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_group_members(group_id,user_id,created_at) VALUES(?,?,?)`), groupID, userID, time.Now().UTC()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ReplaceUserGroupRoles(ctx context.Context, orgID, groupID string, roleKeys []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var groupOrg string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id FROM user_groups WHERE id=? FOR UPDATE`), groupID).Scan(&groupOrg); err != nil {
			return err
		}
		if groupOrg != orgID {
			return sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_group_roles WHERE group_id=?`), groupID); err != nil {
			return err
		}
		for _, roleKey := range roleKeys {
			var roleID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_group_roles(group_id,role_id,created_at) VALUES(?,?,?)`), groupID, roleID, time.Now().UTC()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ListUserAssignments(ctx context.Context, orgID, userID string) ([]identity.UserAssignment, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,user_id,department_id,position_id,is_primary,valid_from,valid_until,created_at FROM user_assignments WHERE organization_id=? AND user_id=? ORDER BY is_primary DESC,valid_from,department_id`), orgID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]identity.UserAssignment, 0)
	for rows.Next() {
		var item identity.UserAssignment
		var position sql.NullString
		var validUntil sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.DepartmentID, &position, &item.Primary, &item.ValidFrom, &validUntil, &item.CreatedAt); err != nil {
			return nil, err
		}
		if position.Valid {
			item.PositionID = position.String
		}
		if validUntil.Valid {
			until := validUntil.Time
			item.ValidUntil = &until
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) CurrentUserDepartmentIDs(ctx context.Context, orgID, userID string, at time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT DISTINCT department_id FROM user_assignments WHERE organization_id=? AND user_id=? AND valid_from<=? AND (valid_until IS NULL OR valid_until>?) ORDER BY department_id`), orgID, userID, at, at)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) DataScopeForUser(ctx context.Context, orgID, userID string) (identity.EffectiveDataScope, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT DISTINCT r.id,r.data_scope_type FROM roles r
		JOIN (
			SELECT role_id FROM user_roles WHERE user_id=?
			UNION
			SELECT ugr.role_id FROM user_group_roles ugr
			JOIN user_groups ug ON ug.id=ugr.group_id AND ug.status='ACTIVE'
			JOIN user_group_members ugm ON ugm.group_id=ug.id
			WHERE ugm.user_id=?
			UNION
			SELECT role_id FROM temporary_role_grants WHERE user_id=? AND revoked_at IS NULL AND valid_from<=? AND valid_until>?
		) effective_roles ON effective_roles.role_id=r.id
		WHERE r.organization_id=?`), userID, userID, userID, time.Now().UTC(), time.Now().UTC(), orgID)
	if err != nil {
		return identity.EffectiveDataScope{}, err
	}
	type roleScope struct{ id, scope string }
	roleScopes := make([]roleScope, 0)
	for rows.Next() {
		var item roleScope
		if err := rows.Scan(&item.id, &item.scope); err != nil {
			_ = rows.Close()
			return identity.EffectiveDataScope{}, err
		}
		roleScopes = append(roleScopes, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return identity.EffectiveDataScope{}, err
	}
	if err := rows.Close(); err != nil {
		return identity.EffectiveDataScope{}, err
	}
	result := identity.EffectiveDataScope{}
	departmentSet := make(map[string]struct{})
	needDepartment := false
	needTree := false
	for _, item := range roleScopes {
		switch item.scope {
		case identity.DataScopeOrganization:
			return identity.EffectiveDataScope{OrganizationWide: true}, nil
		case identity.DataScopeSelf:
			result.Self = true
		case identity.DataScopeDepartment:
			needDepartment = true
		case identity.DataScopeDepartmentTree:
			needDepartment = true
			needTree = true
		case identity.DataScopeCustom:
			ids, err := r.RoleDataScopeDepartments(ctx, orgID, item.id)
			if err != nil {
				return identity.EffectiveDataScope{}, err
			}
			for _, id := range ids {
				departmentSet[id] = struct{}{}
			}
		default:
			return identity.EffectiveDataScope{}, errors.New("invalid role data scope")
		}
	}
	if len(roleScopes) == 0 {
		result.Self = true
	}
	if needDepartment {
		own, err := r.CurrentUserDepartmentIDs(ctx, orgID, userID, time.Now().UTC())
		if err != nil {
			return identity.EffectiveDataScope{}, err
		}
		for _, id := range own {
			departmentSet[id] = struct{}{}
		}
		if needTree && len(own) > 0 {
			departments, err := r.ListDepartments(ctx, orgID)
			if err != nil {
				return identity.EffectiveDataScope{}, err
			}
			children := make(map[string][]string)
			for _, department := range departments {
				if department.Status == "ACTIVE" && department.ParentID != "" {
					children[department.ParentID] = append(children[department.ParentID], department.ID)
				}
			}
			pending := append([]string(nil), own...)
			for len(pending) > 0 {
				current := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				for _, child := range children[current] {
					if _, exists := departmentSet[child]; exists {
						continue
					}
					departmentSet[child] = struct{}{}
					pending = append(pending, child)
				}
			}
		}
	}
	result.DepartmentIDs = make([]string, 0, len(departmentSet))
	for id := range departmentSet {
		result.DepartmentIDs = append(result.DepartmentIDs, id)
	}
	sort.Strings(result.DepartmentIDs)
	return result, nil
}

func (r *IdentityRepo) ReplaceUserAssignments(ctx context.Context, orgID, userID string, assignments []identity.UserAssignment) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id FROM users WHERE id=? FOR UPDATE`), userID).Scan(&actualOrg); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_assignments WHERE organization_id=? AND user_id=?`), orgID, userID); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, assignment := range assignments {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_assignments(id,organization_id,user_id,department_id,position_id,is_primary,valid_from,valid_until,created_at) VALUES(?,?,?,?,?,?,?,?,?)`),
				uuid.NewString(), orgID, userID, assignment.DepartmentID, nullableString(assignment.PositionID), assignment.Primary, assignment.ValidFrom, assignment.ValidUntil, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ListUserSessionIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id FROM sessions WHERE user_id=? AND expires_at>? ORDER BY created_at ASC`), userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) SecuritySettings(ctx context.Context, orgID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT setting_key,setting_value FROM system_settings WHERE organization_id=?`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

func (r *IdentityRepo) SetSecuritySettings(ctx context.Context, orgID, updatedBy string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		upsert := `INSERT INTO system_settings(organization_id,setting_key,setting_value,updated_at,updated_by) VALUES(?,?,?,?,?)
			ON CONFLICT (organization_id, setting_key) DO UPDATE
			SET setting_value=EXCLUDED.setting_value,updated_at=EXCLUDED.updated_at,updated_by=EXCLUDED.updated_by`
		if r.dbProvider() != "postgres" {
			upsert = `INSERT INTO system_settings(organization_id,setting_key,setting_value,updated_at,updated_by) VALUES(?,?,?,?,?)
				ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),updated_at=VALUES(updated_at),updated_by=VALUES(updated_by)`
		}
		q := r.db.Rebind(upsert)
		for key, value := range values {
			if _, err := tx.ExecContext(ctx, q, orgID, key, value, now, updatedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ListPermissions(ctx context.Context) ([]identity.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,permission_key,name,created_at FROM permissions ORDER BY permission_key`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []identity.Permission
	for rows.Next() {
		var item identity.Permission
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) EnsureMenu(ctx context.Context, menu identity.Menu) error {
	var id string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM menus WHERE organization_id=? AND menu_key=?`), menu.OrganizationID, menu.Key).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO menus(id,organization_id,menu_key,parent_key,name,route,icon,permission_key,sort_order,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`), uuid.NewString(), menu.OrganizationID, menu.Key, menu.ParentKey, menu.Name, menu.Route, menu.Icon, menu.PermissionKey, menu.SortOrder, menu.Status, menu.CreatedAt, menu.UpdatedAt)
	if err == nil {
		return nil
	}
	var existing string
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM menus WHERE organization_id=? AND menu_key=?`), menu.OrganizationID, menu.Key).Scan(&existing); readErr == nil {
		return nil
	}
	return err
}

func (r *IdentityRepo) ListMenus(ctx context.Context, orgID string) ([]identity.Menu, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,menu_key,parent_key,name,route,icon,permission_key,sort_order,status,created_at,updated_at FROM menus WHERE organization_id=? AND status='ACTIVE' ORDER BY sort_order,menu_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	menus := make([]identity.Menu, 0)
	for rows.Next() {
		var menu identity.Menu
		if err := rows.Scan(&menu.ID, &menu.OrganizationID, &menu.Key, &menu.ParentKey, &menu.Name, &menu.Route, &menu.Icon, &menu.PermissionKey, &menu.SortOrder, &menu.Status, &menu.CreatedAt, &menu.UpdatedAt); err != nil {
			return nil, err
		}
		menus = append(menus, menu)
	}
	return menus, rows.Err()
}

func (r *IdentityRepo) UpdateMenu(ctx context.Context, orgID string, menu identity.Menu) (identity.Menu, error) {
	var current identity.Menu
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,created_at FROM menus WHERE organization_id=? AND menu_key=?`), orgID, menu.Key).Scan(&current.ID, &current.CreatedAt)
	if err != nil {
		return identity.Menu{}, err
	}
	menu.ID = current.ID
	menu.OrganizationID = orgID
	menu.CreatedAt = current.CreatedAt
	menu.UpdatedAt = time.Now().UTC()
	_, err = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE menus SET parent_key=?,name=?,route=?,icon=?,permission_key=?,sort_order=?,status=?,updated_at=? WHERE organization_id=? AND menu_key=?`), menu.ParentKey, menu.Name, menu.Route, menu.Icon, menu.PermissionKey, menu.SortOrder, menu.Status, menu.UpdatedAt, orgID, menu.Key)
	if err != nil {
		return identity.Menu{}, err
	}
	return menu, nil
}

func (r *IdentityRepo) PermissionsForRole(ctx context.Context, roleID string) ([]identity.Permission, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT p.id,p.permission_key,p.name,p.created_at
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id=p.id
		WHERE rp.role_id=? ORDER BY p.permission_key`), roleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []identity.Permission
	for rows.Next() {
		var item identity.Permission
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) ListRoles(ctx context.Context, orgID string) ([]identity.Role, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,role_key,name,data_scope_type,created_at FROM roles WHERE organization_id=? ORDER BY role_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []identity.Role
	for rows.Next() {
		var item identity.Role
		if err := rows.Scan(&item.ID, &item.Key, &item.Name, &item.DataScope, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range out {
		out[index].Permissions, err = r.PermissionsForRole(ctx, out[index].ID)
		if err != nil {
			return nil, err
		}
		out[index].Departments, err = r.RoleDataScopeDepartments(ctx, orgID, out[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *IdentityRepo) RoleDataScopeDepartments(ctx context.Context, orgID, roleID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT rds.department_id FROM role_data_scope_departments rds JOIN roles r ON r.id=rds.role_id JOIN departments d ON d.id=rds.department_id AND d.organization_id=r.organization_id AND d.status='ACTIVE' WHERE r.organization_id=? AND r.id=? ORDER BY rds.department_id`), orgID, roleID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) ReplaceRoleDataScope(ctx context.Context, orgID, roleKey, scopeType string, departmentIDs []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var roleID string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=? FOR UPDATE`), orgID, roleKey).Scan(&roleID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE roles SET data_scope_type=? WHERE id=?`), scopeType, roleID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM role_data_scope_departments WHERE role_id=?`), roleID); err != nil {
			return err
		}
		for _, departmentID := range departmentIDs {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO role_data_scope_departments(role_id,department_id) VALUES(?,?)`), roleID, departmentID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ReplaceRolePermissions(ctx context.Context, orgID, roleKey string, permissionKeys []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var roleID string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, roleKey).Scan(&roleID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM role_permissions WHERE role_id=?`), roleID); err != nil {
			return err
		}
		for _, key := range permissionKeys {
			var permissionID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM permissions WHERE permission_key=?`), key).Scan(&permissionID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO role_permissions(role_id,permission_id) VALUES(?,?)`), roleID, permissionID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ReplaceUserRoles(ctx context.Context, orgID, userID string, roleKeys []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg, status string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id,status FROM users WHERE id=? FOR UPDATE`), userID).Scan(&actualOrg, &status); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}

		var currentlySystemAdmin int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles ur JOIN roles ro ON ro.id=ur.role_id WHERE ur.user_id=? AND ro.organization_id=? AND ro.role_key='system_admin'`), userID, orgID).Scan(&currentlySystemAdmin); err != nil {
			return err
		}
		newSystemAdmin := false
		for _, key := range roleKeys {
			if key == "system_admin" {
				newSystemAdmin = true
				break
			}
		}
		if status == "ACTIVE" && currentlySystemAdmin > 0 && !newSystemAdmin {
			n, err := lockActiveSystemAdmins(ctx, r.db, tx, orgID)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastSystemAdmin
			}
		}

		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_roles WHERE user_id=?`), userID); err != nil {
			return err
		}
		for _, key := range roleKeys {
			var roleID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), orgID, key).Scan(&roleID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_roles(user_id,role_id) VALUES(?,?)`), userID, roleID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ListSessions(ctx context.Context, orgID string, limit int) ([]identity.Session, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT s.id,u.id,u.login_name,u.display_name,s.expires_at,s.created_at,s.last_seen_at,s.client_ip,s.user_agent
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE u.organization_id=? AND s.expires_at>? ORDER BY s.last_seen_at DESC LIMIT ?`), orgID, time.Now().UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []identity.Session
	for rows.Next() {
		var item identity.Session
		if err := rows.Scan(&item.ID, &item.UserID, &item.LoginName, &item.DisplayName, &item.ExpiresAt, &item.CreatedAt, &item.LastSeenAt, &item.ClientIP, &item.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) RevokeSession(ctx context.Context, orgID, sessionID string) error {
	var actualOrg string
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT u.organization_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=?`), sessionID).Scan(&actualOrg); err != nil {
		return err
	}
	if actualOrg != orgID {
		return sql.ErrNoRows
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE id=?`), sessionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) SetUserStatus(ctx context.Context, orgID, userID, status string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg, currentStatus string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id,status FROM users WHERE id=? FOR UPDATE`), userID).Scan(&actualOrg, &currentStatus); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}
		if currentStatus == "ACTIVE" && status != "ACTIVE" {
			var systemAdmin int
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_roles ur JOIN roles ro ON ro.id=ur.role_id WHERE ur.user_id=? AND ro.organization_id=? AND ro.role_key='system_admin'`), userID, orgID).Scan(&systemAdmin); err != nil {
				return err
			}
			if systemAdmin > 0 {
				n, err := lockActiveSystemAdmins(ctx, r.db, tx, orgID)
				if err != nil {
					return err
				}
				if n <= 1 {
					return ErrLastSystemAdmin
				}
			}
		}
		res, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET status=?,updated_at=? WHERE id=? AND organization_id=?`), status, time.Now().UTC(), userID, orgID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return sql.ErrNoRows
		}
		if status != "ACTIVE" {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=?`), userID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE api_tokens SET revoked_at=? WHERE user_id=? AND revoked_at IS NULL`), time.Now().UTC(), userID); err != nil {
				return err
			}
		}
		return nil
	})
}

func lockActiveSystemAdmins(ctx context.Context, db *database.DB, tx *sql.Tx, orgID string) (int, error) {
	rows, err := tx.QueryContext(ctx, db.Rebind(`SELECT u.id FROM users u JOIN user_roles ur ON ur.user_id=u.id JOIN roles ro ON ro.id=ur.role_id WHERE u.organization_id=? AND u.status='ACTIVE' AND ro.organization_id=? AND ro.role_key='system_admin' FOR UPDATE`), orgID, orgID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		n++
	}
	return n, rows.Err()
}

func (r *IdentityRepo) UnlockUser(ctx context.Context, orgID, userID string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE users SET failed_login_count=0,locked_until=NULL,updated_at=? WHERE id=? AND organization_id=?`), time.Now().UTC(), userID, orgID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) AdminResetPassword(ctx context.Context, orgID, userID, oldHash, newHash string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var actualOrg, actualHash string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id,password_hash FROM users WHERE id=? FOR UPDATE`), userID).Scan(&actualOrg, &actualHash); err != nil {
			return err
		}
		if actualOrg != orgID {
			return sql.ErrNoRows
		}
		if err := ensurePasswordState(oldHash, actualHash); err != nil {
			return err
		}
		now := time.Now().UTC()
		if actualHash != "" {
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO password_history(id,user_id,password_hash,changed_at) VALUES(?,?,?,?)`), uuid.NewString(), userID, actualHash, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET password_hash=?,must_change_password=true,failed_login_count=0,locked_until=NULL,password_changed_at=?,updated_at=? WHERE id=?`), newHash, now, now, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=?`), userID)
		return err
	})
}

func ensurePasswordState(expected, actual string) error {
	if expected != actual {
		return ErrPasswordStateChanged
	}
	return nil
}

func (r *IdentityRepo) MFAEnabled(ctx context.Context, userID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_mfa_factors WHERE user_id=? AND status='ACTIVE'`), userID).Scan(&count)
	return count == 1, err
}

func (r *IdentityRepo) ActiveMFAFactor(ctx context.Context, userID string) (identity.MFAFactor, error) {
	return r.mfaFactor(ctx, userID, "ACTIVE")
}

func (r *IdentityRepo) PendingMFAFactor(ctx context.Context, userID string) (identity.MFAFactor, error) {
	factor, err := r.mfaFactor(ctx, userID, "PENDING")
	if err == nil && !factor.PendingExpiresAt.After(time.Now().UTC()) {
		return identity.MFAFactor{}, sql.ErrNoRows
	}
	return factor, err
}

func (r *IdentityRepo) mfaFactor(ctx context.Context, userID, status string) (identity.MFAFactor, error) {
	var factor identity.MFAFactor
	var confirmed sql.NullTime
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT user_id,status,secret_ciphertext,key_version,pending_expires_at,confirmed_at FROM user_mfa_factors WHERE user_id=? AND status=?`), userID, status).Scan(
		&factor.UserID, &factor.Status, &factor.SecretCiphertext, &factor.KeyVersion, &factor.PendingExpiresAt, &confirmed,
	)
	if confirmed.Valid {
		value := confirmed.Time
		factor.ConfirmedAt = &value
	}
	return factor, err
}

func (r *IdentityRepo) SavePendingMFAFactor(ctx context.Context, userID, ciphertext, keyVersion string, expiresAt time.Time) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var status string
		err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT status FROM user_mfa_factors WHERE user_id=? FOR UPDATE`), userID).Scan(&status)
		switch {
		case err == nil && status == "ACTIVE":
			return ErrMFAAlreadyEnabled
		case err == nil:
			_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE user_mfa_factors SET status='PENDING',secret_ciphertext=?,key_version=?,pending_expires_at=?,confirmed_at=NULL,updated_at=? WHERE user_id=?`), ciphertext, keyVersion, expiresAt, time.Now().UTC(), userID)
			return err
		case !errors.Is(err, sql.ErrNoRows):
			return err
		default:
			now := time.Now().UTC()
			_, err = tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_mfa_factors(user_id,status,secret_ciphertext,key_version,pending_expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`), userID, "PENDING", ciphertext, keyVersion, expiresAt, now, now)
			return err
		}
	})
}

func (r *IdentityRepo) ActivateMFAFactor(ctx context.Context, userID string, recoveryCodeHashes []string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		result, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE user_mfa_factors SET status='ACTIVE',confirmed_at=?,updated_at=? WHERE user_id=? AND status='PENDING' AND pending_expires_at>?`), now, now, userID, now)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			if err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		if _, err = tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM mfa_recovery_codes WHERE user_id=?`), userID); err != nil {
			return err
		}
		for _, hash := range recoveryCodeHashes {
			if _, err = tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO mfa_recovery_codes(id,user_id,code_hash,created_at) VALUES(?,?,?,?)`), uuid.NewString(), userID, hash, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *IdentityRepo) ConsumeMFARecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE mfa_recovery_codes SET used_at=? WHERE user_id=? AND code_hash=? AND used_at IS NULL`), time.Now().UTC(), userID, hash)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *IdentityRepo) DeleteMFAAndOtherSessions(ctx context.Context, userID, currentSessionID string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM user_mfa_factors WHERE user_id=?`), userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`DELETE FROM sessions WHERE user_id=? AND id<>?`), userID, currentSessionID)
		return err
	})
}

func (r *IdentityRepo) EnsureRoleConflict(ctx context.Context, orgID, roleA, roleB, reason string) error {
	if roleB < roleA {
		roleA, roleB = roleB, roleA
	}
	var count int
	if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM role_conflict_rules WHERE organization_id=? AND role_a=? AND role_b=?`), orgID, roleA, roleB).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO role_conflict_rules(id,organization_id,role_a,role_b,reason,status,created_at) VALUES(?,?,?,?,?,'ACTIVE',?)`), uuid.NewString(), orgID, roleA, roleB, reason, time.Now().UTC())
	if err == nil {
		return nil
	}
	if readErr := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM role_conflict_rules WHERE organization_id=? AND role_a=? AND role_b=?`), orgID, roleA, roleB).Scan(&count); readErr == nil && count > 0 {
		return nil
	}
	return err
}

func (r *IdentityRepo) RoleConflictRules(ctx context.Context, orgID string) ([]identity.RoleConflictRule, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,role_a,role_b,reason FROM role_conflict_rules WHERE organization_id=? AND status='ACTIVE' ORDER BY role_a,role_b`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]identity.RoleConflictRule, 0)
	for rows.Next() {
		var rule identity.RoleConflictRule
		if err := rows.Scan(&rule.ID, &rule.OrganizationID, &rule.RoleA, &rule.RoleB, &rule.Reason); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) GroupRolesForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT DISTINCT r.role_key FROM roles r JOIN user_group_roles ugr ON ugr.role_id=r.id JOIN user_groups ug ON ug.id=ugr.group_id AND ug.status='ACTIVE' JOIN user_group_members ugm ON ugm.group_id=ug.id WHERE ugm.user_id=? ORDER BY r.role_key`), userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (r *IdentityRepo) RolesForUserExcludingGroup(ctx context.Context, userID, excludedGroupID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT role_key FROM (
		SELECT r.role_key AS role_key FROM roles r JOIN user_roles ur ON ur.role_id=r.id WHERE ur.user_id=?
		UNION
		SELECT r.role_key AS role_key FROM roles r JOIN user_group_roles ugr ON ugr.role_id=r.id JOIN user_groups ug ON ug.id=ugr.group_id AND ug.status='ACTIVE' JOIN user_group_members ugm ON ugm.group_id=ug.id WHERE ugm.user_id=? AND ug.id<>?
		UNION
		SELECT r.role_key AS role_key FROM roles r JOIN temporary_role_grants trg ON trg.role_id=r.id WHERE trg.user_id=? AND trg.revoked_at IS NULL AND trg.valid_from<=? AND trg.valid_until>?
	) effective_roles ORDER BY role_key`), userID, userID, excludedGroupID, userID, time.Now().UTC(), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}
