package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/reliablemsg"
)

var ErrUserProfileVersionConflict = errors.New("user profile version conflict")
var ErrUserProfileContactConflict = errors.New("user profile contact already exists")

func (r *IdentityRepo) HydrateUserProfile(ctx context.Context, user *identity.User) error {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return sql.ErrNoRows
	}
	var phoneVerified, emailVerified sql.NullTime
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT real_name,gender,phone_country_code,phone,phone_verified_at,email_verified_at,avatar_url,profile_version FROM user_profiles WHERE user_id=?`), user.ID).
		Scan(&user.RealName, &user.Gender, &user.PhoneCountryCode, &user.Phone, &phoneVerified, &emailVerified, &user.AvatarURL, &user.ProfileVersion)
	if errors.Is(err, sql.ErrNoRows) {
		user.Gender = "UNSPECIFIED"
		user.PhoneCountryCode = "+86"
		return nil
	}
	if err != nil {
		return err
	}
	if phoneVerified.Valid {
		value := phoneVerified.Time.UTC()
		user.PhoneVerifiedAt = &value
	}
	if emailVerified.Valid {
		value := emailVerified.Time.UTC()
		user.EmailVerifiedAt = &value
	}
	return nil
}

func (r *IdentityRepo) UpdateUserProfile(ctx context.Context, organizationID, userID, issuer string, input identity.UserProfileInput) (identity.User, error) {
	now := time.Now().UTC()
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var currentEmail, externalSubject, userStatus string
		var provisioningVersion int64
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT email,external_subject,status,provisioning_version FROM users WHERE id=? AND organization_id=? FOR UPDATE`), userID, organizationID).
			Scan(&currentEmail, &externalSubject, &userStatus, &provisioningVersion); err != nil {
			return err
		}
		if input.Email != "" {
			var count int
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM users WHERE organization_id=? AND LOWER(email)=LOWER(?) AND id<>?`), organizationID, input.Email, userID).Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				return ErrUserProfileContactConflict
			}
		}
		if input.Phone != "" {
			var count int
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM user_profiles p JOIN users u ON u.id=p.user_id WHERE u.organization_id=? AND p.phone_country_code=? AND p.phone=? AND p.user_id<>?`), organizationID, input.PhoneCountryCode, input.Phone, userID).Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				return ErrUserProfileContactConflict
			}
		}

		var currentVersion int64
		var currentPhone, currentCountry string
		var phoneVerified, emailVerified sql.NullTime
		readErr := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT profile_version,phone_country_code,phone,phone_verified_at,email_verified_at FROM user_profiles WHERE user_id=? FOR UPDATE`), userID).
			Scan(&currentVersion, &currentCountry, &currentPhone, &phoneVerified, &emailVerified)
		if errors.Is(readErr, sql.ErrNoRows) {
			if input.ExpectedVersion != 0 {
				return ErrUserProfileVersionConflict
			}
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO user_profiles(user_id,real_name,gender,phone_country_code,phone,avatar_url,profile_version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`), userID, input.RealName, input.Gender, input.PhoneCountryCode, input.Phone, input.AvatarURL, 1, now, now); err != nil {
				return err
			}
		} else if readErr != nil {
			return readErr
		} else {
			if input.ExpectedVersion != currentVersion {
				return ErrUserProfileVersionConflict
			}
			if currentPhone != input.Phone || currentCountry != input.PhoneCountryCode {
				phoneVerified = sql.NullTime{}
			}
			if !strings.EqualFold(strings.TrimSpace(currentEmail), input.Email) {
				emailVerified = sql.NullTime{}
			}
			result, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE user_profiles SET real_name=?,gender=?,phone_country_code=?,phone=?,phone_verified_at=?,email_verified_at=?,avatar_url=?,profile_version=profile_version+1,updated_at=? WHERE user_id=? AND profile_version=?`), input.RealName, input.Gender, input.PhoneCountryCode, input.Phone, nullableTime(phoneVerified), nullableTime(emailVerified), input.AvatarURL, now, userID, currentVersion)
			if err != nil {
				return err
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return ErrUserProfileVersionConflict
			}
		}
		if strings.TrimSpace(externalSubject) == "" {
			_, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET display_name=?,email=?,updated_at=? WHERE id=? AND organization_id=?`), input.DisplayName, input.Email, now, userID, organizationID)
			return err
		}

		version := provisioningVersion + 1
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET display_name=?,email=?,provisioning_version=?,updated_at=? WHERE id=? AND organization_id=?`), input.DisplayName, input.Email, version, now, userID, organizationID); err != nil {
			return err
		}
		return r.enqueueProfileProvisioning(ctx, tx, profileProvisioningInput{
			OrganizationID: organizationID, UserID: userID, Subject: externalSubject,
			Issuer: issuer, LoginName: "", DisplayName: input.DisplayName, Email: input.Email,
			UserStatus: userStatus, Version: version, OccurredAt: now,
		})
	})
	if err != nil {
		return identity.User{}, err
	}
	return r.UserByID(ctx, userID)
}

type profileProvisioningInput struct {
	OrganizationID, UserID, Subject, Issuer, LoginName, DisplayName, Email, UserStatus string
	Version                                                                            int64
	OccurredAt                                                                         time.Time
}

// enqueueProfileProvisioning refreshes every existing application projection
// after a managed profile change. It deliberately reuses the stable 1.0 event
// shape so already-integrated applications receive the new display name/email
// without a lock-step receiver upgrade.
func (r *IdentityRepo) enqueueProfileProvisioning(ctx context.Context, tx *sql.Tx, input profileProvisioningInput) error {
	if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT login_name FROM users WHERE id=? AND organization_id=?`), input.UserID, input.OrganizationID).Scan(&input.LoginName); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, r.db.Rebind(`SELECT a.id,a.code,e.roles_json,e.status
		FROM user_application_entitlements e
		JOIN portal_applications a ON a.id=e.application_id
		WHERE e.user_id=? AND a.organization_id=? ORDER BY a.id`), input.UserID, input.OrganizationID)
	if err != nil {
		return err
	}
	type projection struct{ applicationID, applicationCode, rolesJSON, status string }
	items := make([]projection, 0)
	for rows.Next() {
		var item projection
		if err := rows.Scan(&item.applicationID, &item.applicationCode, &item.rolesJSON, &item.status); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		var roles []string
		if err := json.Unmarshal([]byte(item.rolesJSON), &roles); err != nil {
			return err
		}
		status, eventType := item.status, "user.entitlements.changed"
		if input.UserStatus != "ACTIVE" {
			status, roles, eventType = "DISABLED", nil, "user.disabled"
		} else if status != "ACTIVE" {
			status, roles = "DISABLED", nil
		}
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE user_application_entitlements SET version=?,updated_at=? WHERE user_id=? AND application_id=?`), input.Version, input.OccurredAt, input.UserID, item.applicationID); err != nil {
			return err
		}
		event := EntitlementEvent{SchemaVersion: "1.0", EventID: uuid.NewString(), EventType: eventType, AggregateVersion: input.Version, OccurredAt: input.OccurredAt, Source: "velora"}
		event.User.Subject, event.User.Issuer, event.User.LoginName = input.Subject, input.Issuer, input.LoginName
		event.User.DisplayName, event.User.Email, event.User.Status = input.DisplayName, input.Email, status
		event.Entitlements.ApplicationCode, event.Entitlements.Roles = item.applicationCode, roles
		if _, err := reliablemsg.EnqueueTx(ctx, r.db, tx, reliablemsg.Event{ID: event.EventID, OrganizationID: input.OrganizationID, Topic: "velora.provisioning." + item.applicationCode, Key: input.UserID, Type: event.EventType, OrderingKey: input.UserID, Payload: event}); err != nil {
			return err
		}
	}
	return nil
}

func nullableTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}
