package portal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

// #nosec G101 -- this is a public HMAC domain-separation label, not a credential.
const directoryTokenContext = "velora:application-directory:v1:"

type DirectoryAccess struct {
	ApplicationID  string
	OrganizationID string
}

type DirectoryPage struct {
	Users         []portaldomain.DirectoryUser
	NextPageToken string
	SnapshotAt    time.Time
}

type directoryCursor struct {
	SnapshotAt   time.Time `json:"snapshot_at"`
	UpdatedAfter time.Time `json:"updated_after"`
	LastUpdated  time.Time `json:"last_updated"`
	LastID       string    `json:"last_id"`
}

func DeriveDirectoryToken(provisioningSecret, applicationID string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(provisioningSecret)))
	_, _ = mac.Write([]byte(directoryTokenContext + strings.TrimSpace(applicationID)))
	return "vd_" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) AuthenticateDirectoryCredential(ctx context.Context, applicationID, rawToken string) (DirectoryAccess, error) {
	if s.provisioningCipher == nil || strings.TrimSpace(applicationID) == "" || !strings.HasPrefix(strings.TrimSpace(rawToken), "vd_") {
		return DirectoryAccess{}, ErrAccessDenied
	}
	credential, err := s.repo.GetDirectoryCredential(ctx, strings.TrimSpace(applicationID))
	if errors.Is(err, sql.ErrNoRows) {
		return DirectoryAccess{}, ErrAccessDenied
	}
	if err != nil {
		return DirectoryAccess{}, err
	}
	if credential.LifecycleStatus == portaldomain.LifecycleDisabled || !strings.HasPrefix(credential.SecretRef, "enc:") {
		return DirectoryAccess{}, ErrAccessDenied
	}
	aad := []byte("velora:provisioning:" + credential.OrganizationID + ":" + credential.ApplicationCode)
	plain, err := s.provisioningCipher.Decrypt(strings.TrimPrefix(credential.SecretRef, "enc:"), aad)
	if err != nil || len(strings.TrimSpace(string(plain))) < 32 {
		return DirectoryAccess{}, ErrAccessDenied
	}
	expected := DeriveDirectoryToken(string(plain), credential.ApplicationID)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.TrimSpace(rawToken))) != 1 {
		return DirectoryAccess{}, ErrAccessDenied
	}
	return DirectoryAccess{ApplicationID: credential.ApplicationID, OrganizationID: credential.OrganizationID}, nil
}

func (s *Service) DirectoryOrganization(ctx context.Context, access DirectoryAccess) (portaldomain.DirectoryOrganization, error) {
	return s.repo.GetDirectoryOrganization(ctx, access.OrganizationID)
}

func (s *Service) DirectoryDepartments(ctx context.Context, access DirectoryAccess) ([]portaldomain.DirectoryDepartment, error) {
	return s.repo.ListDirectoryDepartments(ctx, access.OrganizationID)
}

func (s *Service) DirectoryUsers(ctx context.Context, access DirectoryAccess, pageSize int, pageToken string, updatedAfter *time.Time) (DirectoryPage, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	cursor := directoryCursor{SnapshotAt: time.Now().UTC(), UpdatedAfter: time.Unix(0, 0).UTC(), LastUpdated: time.Unix(0, 0).UTC()}
	if updatedAfter != nil {
		cursor.UpdatedAfter = updatedAfter.UTC()
		cursor.LastUpdated = cursor.UpdatedAfter
	}
	if strings.TrimSpace(pageToken) != "" {
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(pageToken))
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.SnapshotAt.IsZero() || cursor.LastUpdated.Before(cursor.UpdatedAfter) {
			return DirectoryPage{}, ErrInvalid
		}
	}
	items, err := s.repo.ListDirectoryUsers(ctx, access.OrganizationID, access.ApplicationID, cursor.UpdatedAfter, cursor.SnapshotAt, cursor.LastUpdated, cursor.LastID, pageSize+1)
	if err != nil {
		return DirectoryPage{}, err
	}
	page := DirectoryPage{SnapshotAt: cursor.SnapshotAt}
	if len(items) > pageSize {
		last := items[pageSize-1]
		next := directoryCursor{SnapshotAt: cursor.SnapshotAt, UpdatedAfter: cursor.UpdatedAfter, LastUpdated: last.UpdatedAt, LastID: last.CursorID}
		raw, _ := json.Marshal(next)
		page.NextPageToken = base64.RawURLEncoding.EncodeToString(raw)
		items = items[:pageSize]
	}
	page.Users = items
	return page, nil
}
