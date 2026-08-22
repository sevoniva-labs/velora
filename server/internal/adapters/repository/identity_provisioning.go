package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	identity "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/reliablemsg"
)

type ManagedUserCreate struct {
	OrganizationID, LoginName, DisplayName, Email, PasswordHash, ExternalSubject string
	Roles                                                                        []string
}

func (r *IdentityRepo) CreateManagedUser(ctx context.Context, input ManagedUserCreate) (identity.User, error) {
	now := time.Now().UTC()
	u := identity.User{ID: uuid.NewString(), OrganizationID: input.OrganizationID, LoginName: input.LoginName, DisplayName: input.DisplayName, Email: input.Email, IdentitySource: "CASDOOR", ExternalSubject: input.ExternalSubject, ProvisioningVersion: 1, Status: "ACTIVE", CreatedAt: now, UpdatedAt: now, Roles: input.Roles}
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO users(id,organization_id,login_name,display_name,email,identity_source,external_subject,provisioning_version,password_hash,status,must_change_password,failed_login_count,password_changed_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), u.ID, u.OrganizationID, u.LoginName, u.DisplayName, u.Email, u.IdentitySource, u.ExternalSubject, u.ProvisioningVersion, input.PasswordHash, u.Status, false, 0, now, now, now)
		if err != nil {
			return err
		}
		for _, roleKey := range input.Roles {
			var roleID string
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM roles WHERE organization_id=? AND role_key=?`), u.OrganizationID, roleKey).Scan(&roleID); err != nil {
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

func (r *IdentityRepo) FederatedManagedUser(ctx context.Context, organizationID, provider, subject string) (identity.FederatedIdentityLink, error) {
	if !strings.EqualFold(provider, "casdoor") {
		return identity.FederatedIdentityLink{}, sql.ErrNoRows
	}
	var user identity.User
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,login_name FROM users WHERE organization_id=? AND identity_source='CASDOOR' AND external_subject=?`), organizationID, subject).Scan(&user.ID, &user.OrganizationID, &user.LoginName)
	if err != nil {
		return identity.FederatedIdentityLink{}, err
	}
	return identity.FederatedIdentityLink{ID: "managed:" + user.ID, OrganizationID: user.OrganizationID, Provider: "casdoor", Subject: subject, UserID: user.ID, LoginName: user.LoginName}, nil
}

func (r *IdentityRepo) ListUserEntitlements(ctx context.Context, userID string) ([]identity.ApplicationEntitlement, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT a.code,a.name,e.status,e.roles_json,e.version FROM user_application_entitlements e JOIN portal_applications a ON a.id=e.application_id WHERE e.user_id=? ORDER BY a.name`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]identity.ApplicationEntitlement, 0)
	for rows.Next() {
		var item identity.ApplicationEntitlement
		var raw string
		if err := rows.Scan(&item.ApplicationCode, &item.ApplicationName, &item.Status, &raw, &item.Version); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &item.Roles); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type EntitlementEvent struct {
	SchemaVersion    string    `json:"schema_version"`
	EventID          string    `json:"event_id"`
	EventType        string    `json:"event_type"`
	AggregateVersion int64     `json:"aggregate_version"`
	OccurredAt       time.Time `json:"occurred_at"`
	Source           string    `json:"source"`
	User             struct {
		Subject     string `json:"subject"`
		Issuer      string `json:"issuer"`
		LoginName   string `json:"login_name"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email,omitempty"`
		Status      string `json:"status"`
	} `json:"user"`
	Entitlements struct {
		ApplicationCode string   `json:"application_code"`
		Roles           []string `json:"roles"`
	} `json:"entitlements"`
}

func (r *IdentityRepo) UpsertUserEntitlement(ctx context.Context, actorID, userID, applicationCode, status string, roles []string, issuer string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var u identity.User
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT organization_id,login_name,display_name,email,external_subject,provisioning_version,status FROM users WHERE id=? FOR UPDATE`), userID).Scan(&u.OrganizationID, &u.LoginName, &u.DisplayName, &u.Email, &u.ExternalSubject, &u.ProvisioningVersion, &u.Status); err != nil {
			return err
		}
		if u.ExternalSubject == "" {
			return errors.New("user is not managed by Casdoor")
		}
		var appID, appName string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id,name FROM portal_applications WHERE organization_id=? AND code=?`), u.OrganizationID, applicationCode).Scan(&appID, &appName); err != nil {
			return err
		}
		version := u.ProvisioningVersion + 1
		raw, _ := json.Marshal(roles)
		_, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_application_entitlements(user_id,application_id,roles_json,status,version,updated_by) VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,application_id) DO UPDATE SET roles_json=excluded.roles_json,status=excluded.status,version=excluded.version,updated_by=excluded.updated_by,updated_at=?`), userID, appID, string(raw), status, version, actorID, time.Now().UTC())
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET provisioning_version=?,updated_at=? WHERE id=?`), version, time.Now().UTC(), userID); err != nil {
			return err
		}
		event := EntitlementEvent{SchemaVersion: "1.0", EventID: uuid.NewString(), EventType: "user.entitlements.changed", AggregateVersion: version, OccurredAt: time.Now().UTC(), Source: "velora"}
		event.User.Subject = u.ExternalSubject
		event.User.Issuer = issuer
		event.User.LoginName = u.LoginName
		event.User.DisplayName = u.DisplayName
		event.User.Email = u.Email
		event.User.Status = status
		if u.Status != "ACTIVE" {
			event.User.Status = "DISABLED"
			roles = nil
			event.EventType = "user.disabled"
		}
		event.Entitlements.ApplicationCode = applicationCode
		event.Entitlements.Roles = roles
		_, err = reliablemsg.EnqueueTx(ctx, r.db, tx, reliablemsg.Event{ID: event.EventID, OrganizationID: u.OrganizationID, Topic: "velora.provisioning." + applicationCode, Key: userID, Type: event.EventType, OrderingKey: userID, Payload: event})
		return err
	})
}
