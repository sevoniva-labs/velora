package kratosapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/identitysource"
)

type FederatedLoginOptions struct {
	Cache cache.Cache
	OIDC  map[string]*identitysource.OIDCProvider
	LDAP  map[string]*identitysource.LDAPProvider
}

type FederatedLogin struct {
	cache cache.Cache
	oidc  map[string]*identitysource.OIDCProvider
	ldap  map[string]*identitysource.LDAPProvider
}

func (s *IdentityService) ConfigureFederatedLogin(options FederatedLoginOptions) {
	s.federated = &FederatedLogin{cache: options.Cache, oidc: options.OIDC, ldap: options.LDAP}
}

func (s *IdentityService) BeginOIDCLogin(ctx context.Context, req *forgev1.BeginOIDCLoginRequest) (*forgev1.BeginOIDCLoginResponse, error) {
	if s.federated == nil || s.federated.cache == nil || s.federated.cache.Provider() == "disabled" {
		return nil, federatedUnavailable()
	}
	providerName := normalizeProviderName(req.GetProvider())
	provider, ok := s.federated.oidc[providerName]
	if !ok {
		return nil, kerrors.NotFound("FEDERATED_PROVIDER_NOT_FOUND", "OIDC provider is not configured")
	}
	attempt := newAuditEvent(ctx, domain.Principal{LoginName: providerName}, "auth.federated.begin", "oidc_state", "", nil)
	if err := s.allow(ctx, attempt.ClientIP+"|oidc-begin|"+providerName, 20, time.Minute, "60"); err != nil {
		return nil, err
	}
	state, err := randomFederatedValue()
	if err != nil {
		return nil, federatedUnavailable()
	}
	nonce, err := randomFederatedValue()
	if err != nil {
		return nil, federatedUnavailable()
	}
	organization := strings.TrimSpace(req.GetOrganization())
	if organization == "" {
		organization = "default"
	}
	payload, err := json.Marshal(map[string]string{"provider": providerName, "organization": organization, "nonce": nonce})
	if err != nil {
		return nil, federatedUnavailable()
	}
	if err := s.federated.cache.Set(ctx, "oidc:state:"+state, string(payload), 5*time.Minute); err != nil {
		return nil, federatedUnavailable()
	}
	return &forgev1.BeginOIDCLoginResponse{RedirectUrl: provider.AuthorizationURL(state, nonce)}, nil
}

func (s *IdentityService) CompleteOIDCLogin(ctx context.Context, req *forgev1.CompleteOIDCLoginRequest) (*forgev1.CompleteOIDCLoginResponse, error) {
	if s.federated == nil || s.federated.cache == nil || s.federated.cache.Provider() == "disabled" {
		return nil, federatedUnavailable()
	}
	providerName := normalizeProviderName(req.GetProvider())
	provider, ok := s.federated.oidc[providerName]
	if !ok {
		return nil, kerrors.NotFound("FEDERATED_PROVIDER_NOT_FOUND", "OIDC provider is not configured")
	}
	attempt := newAuditEvent(ctx, domain.Principal{LoginName: providerName}, "auth.federated.callback", "oidc_state", "", nil)
	if err := s.allow(ctx, attempt.ClientIP+"|oidc-callback|"+providerName, 20, time.Minute, "60"); err != nil {
		return nil, err
	}
	state := strings.TrimSpace(req.GetState())
	if state == "" || strings.TrimSpace(req.GetCode()) == "" {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	key := "oidc:state:" + state
	payload, err := s.federated.cache.Get(ctx, key)
	if err != nil {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	var stateData struct {
		Provider     string `json:"provider"`
		Organization string `json:"organization"`
		Nonce        string `json:"nonce"`
	}
	if json.Unmarshal([]byte(payload), &stateData) != nil || stateData.Provider != providerName || stateData.Organization == "" || stateData.Nonce == "" {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	consumed, err := s.federated.cache.CompareAndDelete(ctx, key, payload)
	if err != nil || !consumed {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login state was already used")
	}
	federated, err := provider.AuthenticateCode(ctx, req.GetCode(), stateData.Nonce)
	if err != nil || federated.Provider != providerName || federated.Subject == "" {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	principal, token, csrf, expires, err := s.loginFederated(ctx, stateData.Organization, providerName, federated.Subject)
	if err != nil {
		return nil, err
	}
	s.setLoginCookies(ctx, token, csrf, expires)
	return &forgev1.CompleteOIDCLoginResponse{User: principalUser(principal), CsrfToken: csrf}, nil
}

func (s *IdentityService) LoginLDAP(ctx context.Context, req *forgev1.LoginLDAPRequest) (*forgev1.LoginLDAPResponse, error) {
	if s.federated == nil {
		return nil, federatedUnavailable()
	}
	providerName := normalizeProviderName(req.GetProvider())
	provider, ok := s.federated.ldap[providerName]
	if !ok {
		return nil, kerrors.NotFound("FEDERATED_PROVIDER_NOT_FOUND", "LDAP provider is not configured")
	}
	organization := strings.TrimSpace(req.GetOrganization())
	if organization == "" {
		organization = "default"
	}
	attempt := domain.Principal{LoginName: strings.TrimSpace(req.GetLoginName())}
	event := newAuditEvent(ctx, attempt, "auth.federated.login", "session", "", map[string]any{"provider": providerName})
	event.Result = "FAILED"
	if err := s.allow(ctx, event.ClientIP+"|federated|"+providerName, 10, time.Minute, "60"); err != nil {
		return nil, err
	}
	federated, err := provider.Authenticate(ctx, req.GetLoginName(), req.GetPassword())
	if err != nil || federated.Provider != providerName || federated.Subject == "" {
		_ = s.audit.Write(ctx, *event)
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	principal, token, csrf, expires, err := s.loginFederated(ctx, organization, providerName, federated.Subject)
	if err != nil {
		return nil, err
	}
	s.setLoginCookies(ctx, token, csrf, expires)
	return &forgev1.LoginLDAPResponse{User: principalUser(principal), CsrfToken: csrf}, nil
}

func (s *IdentityService) loginFederated(ctx context.Context, organization, provider, subject string) (domain.Principal, string, string, time.Time, error) {
	var principal domain.Principal
	var token, csrf string
	var expires time.Time
	var loginErr error
	event := newAuditEvent(ctx, domain.Principal{LoginName: provider}, "auth.federated.login", "session", "", map[string]any{"provider": provider})
	event.Result = "FAILED"
	txErr := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		var organizationID string
		if err := s.db.QueryRowContext(txCtx, s.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), strings.TrimSpace(organization)).Scan(&organizationID); err != nil {
			loginErr = appidentity.ErrInvalidCredentials
			return s.audit.Write(txCtx, *event)
		}
		event.OrganizationID = organizationID
		principal, token, csrf, expires, loginErr = s.identity.LoginFederated(txCtx, organizationID, provider, subject, event.ClientIP, requestHeader(ctx, "User-Agent", 512))
		if loginErr == nil {
			event.ActorID, event.ActorName, event.ResourceID, event.Result = principal.UserID, principal.LoginName, principal.SessionID, "SUCCESS"
		}
		if loginErr != nil && !errors.Is(loginErr, appidentity.ErrInvalidCredentials) && !errors.Is(loginErr, appidentity.ErrDisabled) {
			return loginErr
		}
		return s.audit.Write(txCtx, *event)
	})
	if txErr != nil {
		return domain.Principal{}, "", "", time.Time{}, internalError(txErr)
	}
	if loginErr != nil {
		return domain.Principal{}, "", "", time.Time{}, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	return principal, token, csrf, expires, nil
}

func normalizeProviderName(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func randomFederatedValue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func federatedUnavailable() error {
	return kerrors.ServiceUnavailable("FEDERATED_UNAVAILABLE", "federated identity provider is unavailable")
}
