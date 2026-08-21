package transport

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	identitydomain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

const minimumReferenceTokenBytes = 32

type TokenAuthenticator struct {
	digest         [sha256.Size]byte
	subject        string
	organizationID string
}

func NewTokenAuthenticator(digestHex, subject, organizationID string) (*TokenAuthenticator, error) {
	rawDigest, err := hex.DecodeString(strings.TrimSpace(digestHex))
	if err != nil || len(rawDigest) != sha256.Size {
		return nil, errors.New("reference service token SHA-256 digest must be 64 hexadecimal characters")
	}
	subject = strings.TrimSpace(subject)
	organizationID = strings.TrimSpace(organizationID)
	if subject == "" || organizationID == "" {
		return nil, errors.New("reference service token subject and organization are required")
	}
	authenticator := &TokenAuthenticator{subject: subject, organizationID: organizationID}
	copy(authenticator.digest[:], rawDigest)
	return authenticator, nil
}

func (*TokenAuthenticator) Authenticate(context.Context, string) (identitydomain.Principal, error) {
	return identitydomain.Principal{}, errors.New("reference service does not accept browser sessions")
}

func (a *TokenAuthenticator) AuthenticateAPIToken(ctx context.Context, token string) (identitydomain.Principal, error) {
	if err := ctx.Err(); err != nil {
		return identitydomain.Principal{}, err
	}
	if len(token) < minimumReferenceTokenBytes {
		return identitydomain.Principal{}, errors.New("invalid reference service token")
	}
	actual := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(actual[:], a.digest[:]) != 1 {
		return identitydomain.Principal{}, errors.New("invalid reference service token")
	}
	return identitydomain.Principal{
		Type: "TOKEN", UserID: a.subject, LoginName: a.subject, DisplayName: a.subject,
		OrganizationID: a.organizationID, Roles: []string{"service_client"},
		Permissions: []string{PermissionReadSettlement}, Scopes: []string{PermissionReadSettlement},
	}, nil
}
