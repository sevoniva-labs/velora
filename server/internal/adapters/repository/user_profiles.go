package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/domain/identity"
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

func (r *IdentityRepo) UpdateUserProfile(ctx context.Context, organizationID, userID string, input identity.UserProfileInput) (identity.User, error) {
	now := time.Now().UTC()
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var currentEmail string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT email FROM users WHERE id=? AND organization_id=? FOR UPDATE`), userID, organizationID).Scan(&currentEmail); err != nil {
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
		_, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE users SET display_name=?,email=?,updated_at=? WHERE id=? AND organization_id=?`), input.DisplayName, input.Email, now, userID, organizationID)
		return err
	})
	if err != nil {
		return identity.User{}, err
	}
	return r.UserByID(ctx, userID)
}

func nullableTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}
