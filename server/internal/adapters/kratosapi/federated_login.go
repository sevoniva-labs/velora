package kratosapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
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

func (s *IdentityService) loginCasdoorPassword(ctx context.Context, req *forgev1.LoginRequest, event *audit.Event) (*forgev1.LoginResponse, error) {
	if s.casdoorPasswordProvider == nil {
		return nil, federatedUnavailable()
	}
	organization := strings.TrimSpace(req.GetOrganization())
	if organization == "" {
		organization = "default"
	}
	// Velora owns the optional second factor shown on the portal. Do not forward
	// its TOTP/recovery code to Casdoor: the provider only authenticates the
	// primary enterprise credential, then loginFederated verifies Velora MFA.
	federated, err := s.casdoorPasswordProvider.AuthenticatePassword(ctx, req.GetLoginName(), req.GetPassword(), "", "")
	if err != nil || federated.Provider != s.casdoorPasswordProvider.Name() || federated.Subject == "" {
		_ = s.audit.Write(ctx, *event)
		if challengeErr := s.markLoginChallenge(ctx, event.ClientIP, req.GetLoginName()); challengeErr != nil {
			return nil, kerrors.ServiceUnavailable("LOGIN_CHALLENGE_UNAVAILABLE", "login security state is unavailable")
		}
		return nil, kerrors.Forbidden("TURNSTILE_REQUIRED", "security verification is required")
	}
	principal, token, csrf, expires, err := s.loginFederated(ctx, organization, federated.Provider, federated.Subject, federated.MFAVerified, req.GetMfaCode(), req.GetRecoveryCode())
	if err != nil {
		_ = s.audit.Write(ctx, *event)
		return nil, err
	}
	response := &forgev1.LoginResponse{User: principalUser(principal), CsrfToken: csrf}
	if s.sessionBridge != nil {
		if strings.TrimSpace(federated.CasdoorSessionCookie) == "" {
			return nil, federatedUnavailable()
		}
		ticket, err := s.sessionBridge.Create(ctx, federated.CasdoorSessionCookie, req.GetReturnPath(), principal)
		if err != nil {
			return nil, federatedUnavailable()
		}
		response.BridgeAction = s.sessionBridge.ActionURL()
		response.BridgeTicket = ticket
	}
	s.clearLoginChallenge(ctx, event.ClientIP, req.GetLoginName())
	s.setLoginCookies(ctx, token, csrf, expires)
	return response, nil
}

const oidcTransactionCookieName = "velora_oidc_tx"

const (
	maxFederatedProvider = 64
	maxFederatedOrg      = 100
	maxOIDCCode          = 4096
	maxOIDCState         = 512
	maxLDAPLogin         = 120
	maxLDAPPassword      = 512
)

func (s *IdentityService) ConfigureFederatedLogin(options FederatedLoginOptions) {
	s.federated = &FederatedLogin{cache: options.Cache, oidc: options.OIDC, ldap: options.LDAP}
}

func (s *IdentityService) BeginOIDCLogin(ctx context.Context, req *forgev1.BeginOIDCLoginRequest) (*forgev1.BeginOIDCLoginResponse, error) {
	if s.federated == nil || s.federated.cache == nil || s.federated.cache.Provider() == "disabled" {
		return nil, federatedUnavailable()
	}
	providerName := normalizeProviderName(req.GetProvider())
	if len(providerName) == 0 || len(providerName) > maxFederatedProvider {
		return nil, kerrors.New(http.StatusBadRequest, "FEDERATED_PROVIDER_INVALID", "OIDC provider is invalid")
	}
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
	if len(organization) > maxFederatedOrg {
		return nil, kerrors.New(http.StatusBadRequest, "FEDERATED_ORGANIZATION_INVALID", "organization is invalid")
	}
	verifier, err := randomPKCEVerifier()
	if err != nil {
		return nil, federatedUnavailable()
	}
	challenge := pkceChallenge(verifier)
	payload, err := json.Marshal(map[string]string{"provider": providerName, "organization": organization, "nonce": nonce, "verifier": verifier})
	if err != nil {
		return nil, federatedUnavailable()
	}
	if err := s.federated.cache.Set(ctx, oidcTransactionKey(state), string(payload), 5*time.Minute); err != nil {
		return nil, federatedUnavailable()
	}
	setOIDCTransactionCookie(ctx, state, s.secure)
	return &forgev1.BeginOIDCLoginResponse{RedirectUrl: provider.AuthorizationURL(state, nonce, challenge)}, nil
}

func (s *IdentityService) CompleteOIDCLogin(ctx context.Context, req *forgev1.CompleteOIDCLoginRequest) (*forgev1.CompleteOIDCLoginResponse, error) {
	if s.federated == nil || s.federated.cache == nil || s.federated.cache.Provider() == "disabled" {
		return nil, federatedUnavailable()
	}
	providerName := normalizeProviderName(req.GetProvider())
	if len(providerName) == 0 || len(providerName) > maxFederatedProvider || len(req.GetCode()) > maxOIDCCode || len(req.GetState()) > maxOIDCState {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
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
		clearOIDCTransactionCookie(ctx, s.secure)
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	defer clearOIDCTransactionCookie(ctx, s.secure)
	cookieState := requestCookie(ctx, oidcTransactionCookieName)
	if cookieState == "" || subtle.ConstantTimeCompare([]byte(cookieState), []byte(state)) != 1 {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login transaction is invalid")
	}
	key := oidcTransactionKey(state)
	payload, err := s.federated.cache.Get(ctx, key)
	if err != nil {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	var stateData struct {
		Provider     string `json:"provider"`
		Organization string `json:"organization"`
		Nonce        string `json:"nonce"`
		Verifier     string `json:"verifier"`
	}
	if json.Unmarshal([]byte(payload), &stateData) != nil || stateData.Provider != providerName || stateData.Organization == "" || stateData.Nonce == "" || stateData.Verifier == "" {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	consumed, err := s.federated.cache.CompareAndDelete(ctx, key, payload)
	if err != nil || !consumed {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login state was already used")
	}
	federated, err := provider.AuthenticateCode(ctx, req.GetCode(), stateData.Nonce, stateData.Verifier)
	if err != nil || federated.Provider != providerName || federated.Subject == "" {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
	}
	principal, token, csrf, expires, err := s.loginFederated(ctx, stateData.Organization, providerName, federated.Subject, federated.MFAVerified, "", "")
	if err != nil {
		return nil, err
	}
	s.setLoginCookies(ctx, token, csrf, expires)
	return &forgev1.CompleteOIDCLoginResponse{User: principalUser(principal), CsrfToken: csrf}, nil
}

func (s *IdentityService) LoginLDAP(ctx context.Context, req *forgev1.LoginLDAPRequest) (*forgev1.LoginLDAPResponse, error) {
	if !s.passwordLoginEnabled {
		return nil, kerrors.ServiceUnavailable("PASSWORD_LOGIN_DISABLED", "password login is disabled; use the configured OIDC provider")
	}
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
	if len(organization) > maxFederatedOrg || len(req.GetLoginName()) > maxLDAPLogin || len(req.GetPassword()) > maxLDAPPassword {
		return nil, kerrors.Unauthorized("FEDERATED_LOGIN_FAILED", "federated login failed")
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
	principal, token, csrf, expires, err := s.loginFederated(ctx, organization, providerName, federated.Subject, federated.MFAVerified, "", "")
	if err != nil {
		return nil, err
	}
	s.setLoginCookies(ctx, token, csrf, expires)
	return &forgev1.LoginLDAPResponse{User: principalUser(principal), CsrfToken: csrf}, nil
}

func (s *IdentityService) loginFederated(ctx context.Context, organization, provider, subject string, mfaVerified bool, mfaCode, recoveryCode string) (domain.Principal, string, string, time.Time, error) {
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
		principal, token, csrf, expires, loginErr = s.identity.LoginFederated(txCtx, organizationID, provider, subject, event.ClientIP, requestHeader(ctx, "User-Agent", 512), mfaVerified, mfaCode, recoveryCode)
		if loginErr == nil {
			event.ActorID, event.ActorName, event.ResourceID, event.Result = principal.UserID, principal.LoginName, principal.SessionID, "SUCCESS"
		}
		if loginErr != nil && !errors.Is(loginErr, appidentity.ErrInvalidCredentials) && !errors.Is(loginErr, appidentity.ErrDisabled) && !errors.Is(loginErr, appidentity.ErrMFARequired) && !errors.Is(loginErr, appidentity.ErrInvalidMFA) {
			return loginErr
		}
		return s.audit.Write(txCtx, *event)
	})
	if txErr != nil {
		return domain.Principal{}, "", "", time.Time{}, internalError(txErr)
	}
	if errors.Is(loginErr, appidentity.ErrMFARequired) {
		return domain.Principal{}, "", "", time.Time{}, kerrors.New(http.StatusPreconditionRequired, "MFA_REQUIRED", "multi-factor authentication is required")
	}
	if errors.Is(loginErr, appidentity.ErrInvalidMFA) {
		return domain.Principal{}, "", "", time.Time{}, kerrors.Unauthorized("INVALID_MFA", "invalid multi-factor authentication code")
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

func randomPKCEVerifier() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// oidcTransactionKey keeps the high-entropy browser state out of cache key
// listings and metrics while preserving a deterministic one-time lookup.
func oidcTransactionKey(state string) string {
	sum := sha256.Sum256([]byte(state))
	return "oidc:state:" + hex.EncodeToString(sum[:])
}

func setOIDCTransactionCookie(ctx context.Context, state string, secure bool) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr.Kind() != transport.KindHTTP {
		return
	}
	// #nosec G124 -- state is one-time, HttpOnly and SameSite protected; Secure is enforced in production config.
	tr.ReplyHeader().Add("Set-Cookie", (&http.Cookie{Name: oidcTransactionCookieName, Value: state, Path: "/api/v1/auth/federated/oidc", MaxAge: 300, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode}).String())
}

func clearOIDCTransactionCookie(ctx context.Context, secure bool) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr.Kind() != transport.KindHTTP {
		return
	}
	// #nosec G124 -- deletion cookie mirrors the validated runtime policy.
	tr.ReplyHeader().Add("Set-Cookie", (&http.Cookie{Name: oidcTransactionCookieName, Path: "/api/v1/auth/federated/oidc", MaxAge: -1, Expires: time.Unix(0, 0), HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode}).String())
}

func federatedUnavailable() error {
	return kerrors.ServiceUnavailable("FEDERATED_UNAVAILABLE", "federated identity provider is unavailable")
}
