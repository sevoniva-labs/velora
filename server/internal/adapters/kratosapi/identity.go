package kratosapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/identitysource"
	"github.com/sevoniva-labs/velora/server/internal/platform/ratelimit"
)

const (
	sessionCookieName = "velora_session"
	csrfCookieName    = "velora_csrf"
)

type IdentityService struct {
	forgev1.UnimplementedIdentityServiceServer
	identity                    *appidentity.Service
	audit                       *audit.Writer
	db                          *database.DB
	limiter                     *ratelimit.Limiter
	secure                      bool
	sameSite                    http.SameSite
	passwordLoginEnabled        bool
	casdoorPasswordLoginEnabled bool
	casdoorPasswordProvider     *identitysource.OIDCProvider
	federated                   *FederatedLogin
}

func NewIdentityService(identity *appidentity.Service, auditWriter *audit.Writer, db *database.DB, limiter *ratelimit.Limiter, secureCookies bool, sameSite string) *IdentityService {
	return &IdentityService{identity: identity, audit: auditWriter, db: db, limiter: limiter, secure: secureCookies, sameSite: parseSameSite(sameSite), passwordLoginEnabled: true}
}

// ConfigureAuthMode disables password entry in production OIDC mode. Local
// development keeps the password flow available for bootstrap and testing.
func (s *IdentityService) ConfigureAuthMode(mode string) {
	mode = strings.TrimSpace(mode)
	s.passwordLoginEnabled = mode == "" || strings.EqualFold(mode, "password")
}

func (s *IdentityService) ConfigureCasdoorPasswordLogin(enabled bool, provider *identitysource.OIDCProvider) {
	s.casdoorPasswordLoginEnabled = enabled && provider != nil
	s.casdoorPasswordProvider = provider
}

func (s *IdentityService) requirePasswordManagement() error {
	if !s.passwordLoginEnabled {
		return kerrors.ServiceUnavailable("PASSWORD_MANAGEMENT_DISABLED", "password and local MFA management are disabled; use the configured OIDC provider")
	}
	return nil
}

func (s *IdentityService) Login(ctx context.Context, req *forgev1.LoginRequest) (*forgev1.LoginResponse, error) {
	if !s.passwordLoginEnabled && !s.casdoorPasswordLoginEnabled {
		return nil, kerrors.ServiceUnavailable("PASSWORD_LOGIN_DISABLED", "password login is disabled; use the configured OIDC provider")
	}
	if len(req.GetLoginName()) > 120 || len(req.GetPassword()) > 512 || len(req.GetMfaCode()) > 32 || len(req.GetRecoveryCode()) > 128 {
		return nil, kerrors.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
	}
	loginName := strings.TrimSpace(req.GetLoginName())
	attempt := domain.Principal{LoginName: loginName}
	event := newAuditEvent(ctx, attempt, "auth.login", "session", "", nil)
	event.Result = "FAILED"
	for _, key := range loginRateLimitKeys(event.ClientIP, loginName) {
		if err := s.allow(ctx, key, 10, time.Minute, "60"); err != nil {
			return nil, err
		}
	}
	if s.casdoorPasswordLoginEnabled {
		return s.loginCasdoorPassword(ctx, req, event)
	}

	organization := strings.TrimSpace(req.GetOrganization())
	if organization == "" {
		organization = "default"
	}
	var organizationID string
	if err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT id FROM organizations WHERE org_key=?`), organization).Scan(&organizationID); err != nil {
		if auditErr := s.audit.Write(ctx, *event); auditErr != nil {
			return nil, internalError(auditErr)
		}
		return nil, kerrors.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
	}

	var principal domain.Principal
	var sessionToken, csrfToken string
	var expiresAt time.Time
	var loginErr error
	event.OrganizationID = organizationID
	txErr := s.db.WithinTx(ctx, func(txCtx context.Context) error {
		principal, sessionToken, csrfToken, expiresAt, loginErr = s.identity.LoginWithMFA(
			txCtx, organizationID, req.GetLoginName(), req.GetPassword(), req.GetMfaCode(), req.GetRecoveryCode(), event.ClientIP, requestHeader(ctx, "User-Agent", 512),
		)
		if loginErr != nil && !expectedLoginFailure(loginErr) {
			return loginErr
		}
		if loginErr == nil {
			event.ActorID = principal.UserID
			event.ActorName = principal.LoginName
			event.ResourceID = principal.SessionID
			event.Result = "SUCCESS"
		}
		return s.audit.Write(txCtx, *event)
	})
	if txErr != nil {
		return nil, internalError(txErr)
	}
	if loginErr != nil {
		if errors.Is(loginErr, appidentity.ErrMFARequired) {
			return nil, kerrors.New(http.StatusPreconditionRequired, "MFA_REQUIRED", "multi-factor authentication is required")
		}
		if errors.Is(loginErr, appidentity.ErrInvalidMFA) {
			return nil, kerrors.Unauthorized("INVALID_MFA", "invalid multi-factor authentication code")
		}
		if errors.Is(loginErr, appidentity.ErrLocked) {
			return nil, kerrors.New(http.StatusLocked, "ACCOUNT_LOCKED", "account is locked")
		}
		return nil, kerrors.Unauthorized("INVALID_CREDENTIALS", "invalid credentials")
	}
	s.setLoginCookies(ctx, sessionToken, csrfToken, expiresAt)
	return &forgev1.LoginResponse{User: principalUser(principal), CsrfToken: csrfToken}, nil
}

// loginRateLimitKeys applies independent IP and normalized-account windows.
// The account component is hashed so raw login identifiers never become cache
// keys or appear in rate-limit telemetry.
func loginRateLimitKeys(clientIP, loginName string) []string {
	normalized := strings.ToLower(strings.TrimSpace(loginName))
	digest := sha256.Sum256([]byte(normalized))
	return []string{
		clientIP + "|login-ip",
		"login-account:" + hex.EncodeToString(digest[:]),
	}
}

func (s *IdentityService) GetMFAStatus(ctx context.Context, _ *forgev1.GetMFAStatusRequest) (*forgev1.GetMFAStatusResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePasswordManagement(); err != nil {
		return nil, err
	}
	enabled, err := s.identity.MFAEnabled(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.GetMFAStatusResponse{Enabled: enabled}, nil
}

func (s *IdentityService) BeginMFAEnrollment(ctx context.Context, req *forgev1.BeginMFAEnrollmentRequest) (*forgev1.BeginMFAEnrollmentResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePasswordManagement(); err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "auth.mfa.enrollment.begin", "user", principal.UserID, nil)
	var enrollment domain.MFAEnrollment
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var enrollErr error
		enrollment, enrollErr = s.identity.BeginMFAEnrollment(txCtx, principal, req.GetCurrentPassword())
		return enrollErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.BeginMFAEnrollmentResponse{Secret: enrollment.Secret, ProvisioningUri: enrollment.URL}, nil
}

func (s *IdentityService) ConfirmMFAEnrollment(ctx context.Context, req *forgev1.ConfirmMFAEnrollmentRequest) (*forgev1.ConfirmMFAEnrollmentResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePasswordManagement(); err != nil {
		return nil, err
	}
	if err := s.allow(ctx, "mfa-confirm:user:"+principal.UserID, 5, 5*time.Minute, "300"); err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "auth.mfa.enrollment.confirm", "user", principal.UserID, nil)
	var recoveryCodes []string
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var confirmErr error
		recoveryCodes, confirmErr = s.identity.ConfirmMFAEnrollment(txCtx, principal, req.GetCode())
		return confirmErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ConfirmMFAEnrollmentResponse{RecoveryCodes: recoveryCodes}, nil
}

func (s *IdentityService) DisableMFA(ctx context.Context, req *forgev1.DisableMFARequest) (*forgev1.DisableMFAResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePasswordManagement(); err != nil {
		return nil, err
	}
	if err := s.allow(ctx, "mfa-disable:user:"+principal.UserID, 5, 15*time.Minute, "900"); err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "auth.mfa.disable", "user", principal.UserID, nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.DisableMFA(txCtx, principal, req.GetCurrentPassword(), req.GetCode(), req.GetRecoveryCode())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.DisableMFAResponse{}, nil
}

func (s *IdentityService) Logout(ctx context.Context, _ *forgev1.LogoutRequest) (*forgev1.LogoutResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(principal.Type, "TOKEN") {
		return nil, serviceError(appidentity.ErrInteractiveSessionRequired)
	}
	event := newAuditEvent(ctx, principal, "auth.logout", "session", principal.SessionID, nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.Logout(txCtx, requestCookie(ctx, sessionCookieName))
	})
	if err != nil {
		return nil, serviceError(err)
	}
	s.clearLoginCookies(ctx)
	var federatedLogoutURL string
	if strings.EqualFold(principal.AuthenticationLevel, "FEDERATED") && s.federated != nil {
		// Casdoor is the sole production OIDC provider. Local session
		// revocation above is authoritative; this optional URL clears the
		// upstream browser session when the provider supports RP-initiated
		// logout. Never fail local logout merely because discovery omits it.
		if provider, ok := s.federated.oidc["casdoor"]; ok {
			federatedLogoutURL, _ = provider.EndSessionURL()
		}
	}
	return &forgev1.LogoutResponse{FederatedLogoutUrl: federatedLogoutURL}, nil
}

func (s *IdentityService) GetCurrentUser(ctx context.Context, _ *forgev1.GetCurrentUserRequest) (*forgev1.GetCurrentUserResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	return &forgev1.GetCurrentUserResponse{User: principalUser(principal)}, nil
}

func (s *IdentityService) ChangePassword(ctx context.Context, req *forgev1.ChangePasswordRequest) (*forgev1.ChangePasswordResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePasswordManagement(); err != nil {
		return nil, err
	}
	for _, key := range []string{"password-change:user:" + principal.UserID, "password-change:ip:" + clientIP(ctx)} {
		if err := s.allow(ctx, key, 5, 15*time.Minute, "900"); err != nil {
			return nil, err
		}
	}
	event := newAuditEvent(ctx, principal, "auth.password.change", "user", principal.UserID, nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.ChangePassword(txCtx, principal, req.GetCurrentPassword(), req.GetNewPassword())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ChangePasswordResponse{}, nil
}

func (s *IdentityService) StepUpAuthentication(ctx context.Context, req *forgev1.StepUpAuthenticationRequest) (*forgev1.StepUpAuthenticationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requirePasswordManagement(); err != nil {
		return nil, err
	}
	if err := s.allow(ctx, "step-up:user:"+principal.UserID, 5, 5*time.Minute, "300"); err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "auth.step_up", "session", principal.SessionID, nil)
	var verifiedAt time.Time
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var stepUpErr error
		verifiedAt, stepUpErr = s.identity.StepUpAuthentication(txCtx, principal, req.GetCurrentPassword(), req.GetMfaCode(), req.GetRecoveryCode())
		return stepUpErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.StepUpAuthenticationResponse{VerifiedAt: timestamp(verifiedAt)}, nil
}

func (s *IdentityService) ListApiTokens(ctx context.Context, _ *forgev1.ListApiTokensRequest) (*forgev1.ListApiTokensResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	tokens, err := s.identity.ListAPITokens(ctx, principal)
	if err != nil {
		return nil, serviceError(err)
	}
	reply := &forgev1.ListApiTokensResponse{Tokens: make([]*forgev1.ApiToken, 0, len(tokens))}
	for _, token := range tokens {
		reply.Tokens = append(reply.Tokens, apiTokenProto(token))
	}
	return reply, nil
}

func (s *IdentityService) CreateApiToken(ctx context.Context, req *forgev1.CreateApiTokenRequest) (*forgev1.CreateApiTokenResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	days := req.GetExpiresDays()
	if days == 0 {
		days = 90
	}
	if days < 1 || days > 365 {
		return nil, serviceError(appidentity.ErrInvalidSecurityPolicy)
	}
	var token domain.APIToken
	var secret string
	event := newAuditEvent(ctx, principal, "security.api_token.create", "api_token", "", map[string]any{"name": req.GetName(), "scopes": req.GetScopes()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		token, secret, createErr = s.identity.CreateAPIToken(txCtx, principal, req.GetName(), req.GetScopes(), time.Duration(days)*24*time.Hour)
		if createErr == nil {
			event.ResourceID = token.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateApiTokenResponse{Token: apiTokenProto(token), Secret: secret}, nil
}

func (s *IdentityService) RevokeApiToken(ctx context.Context, req *forgev1.RevokeApiTokenRequest) (*forgev1.RevokeApiTokenResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "security.api_token.revoke", "api_token", req.GetTokenId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.RevokeAPIToken(txCtx, principal, req.GetTokenId())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeApiTokenResponse{}, nil
}

func (s *IdentityService) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
	platform := PlatformService{audit: s.audit, db: s.db}
	return platform.audited(ctx, event, operation)
}

func apiTokenProto(token domain.APIToken) *forgev1.ApiToken {
	return &forgev1.ApiToken{
		Id: token.ID, Name: token.Name, Prefix: token.Prefix, Scopes: token.Scopes,
		CreatedAt: timestamp(token.CreatedAt), ExpiresAt: optionalTimestamp(token.ExpiresAt), LastUsedAt: optionalTimestamp(token.LastUsedAt),
	}
}

func principalUser(principal domain.Principal) *forgev1.User {
	return &forgev1.User{
		Id: principal.UserID, OrganizationId: principal.OrganizationID, LoginName: principal.LoginName,
		DisplayName: principal.DisplayName, MustChangePassword: principal.MustChangePassword,
		PasswordChangedAt: timestamp(principal.PasswordChangedAt), Roles: principal.Roles, Permissions: principal.Permissions,
		DataScope:           effectiveDataScopeProto(principal.DataScope),
		AuthenticationLevel: principal.AuthenticationLevel, MfaVerifiedAt: optionalTimestamp(principal.MFAVerifiedAt),
	}
}

func effectiveDataScopeProto(scope domain.EffectiveDataScope) *forgev1.EffectiveDataScope {
	return &forgev1.EffectiveDataScope{OrganizationWide: scope.OrganizationWide, Self: scope.Self, DepartmentIds: scope.DepartmentIDs}
}

func (s *IdentityService) allow(ctx context.Context, key string, limit int, window time.Duration, retryAfter string) error {
	if s.limiter == nil {
		return kerrors.ServiceUnavailable("RATE_LIMIT_UNAVAILABLE", "security rate limiter is unavailable")
	}
	allowed, err := s.limiter.Allow(ctx, key, limit, window, time.Now())
	if err != nil {
		return kerrors.ServiceUnavailable("RATE_LIMIT_UNAVAILABLE", "security rate limiter is unavailable")
	}
	if allowed {
		return nil
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		tr.ReplyHeader().Set("Retry-After", retryAfter)
	}
	return kerrors.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
}

func (s *IdentityService) setLoginCookies(ctx context.Context, session, csrf string, expires time.Time) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr.Kind() != transport.KindHTTP {
		return
	}
	// #nosec G124 -- production validation requires Secure; local HTTP development remains explicitly supported.
	tr.ReplyHeader().Add("Set-Cookie", (&http.Cookie{
		Name: sessionCookieName, Value: session, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.secure, SameSite: s.sameSite,
	}).String())
	// #nosec G124 -- the double-submit CSRF cookie must be browser-readable and carries no authentication secret.
	tr.ReplyHeader().Add("Set-Cookie", (&http.Cookie{
		Name: csrfCookieName, Value: csrf, Path: "/", Expires: expires,
		Secure: s.secure, SameSite: s.sameSite,
	}).String())
}

func (s *IdentityService) clearLoginCookies(ctx context.Context) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok || tr.Kind() != transport.KindHTTP {
		return
	}
	for _, item := range []struct {
		name     string
		httpOnly bool
	}{{sessionCookieName, true}, {csrfCookieName, false}} {
		// #nosec G124 -- deletion cookies mirror the validated runtime policy and contain no secret value.
		tr.ReplyHeader().Add("Set-Cookie", (&http.Cookie{
			Name: item.name, Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
			HttpOnly: item.httpOnly, Secure: s.secure, SameSite: s.sameSite,
		}).String())
	}
}

func requestCookie(ctx context.Context, name string) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	request := &http.Request{Header: http.Header{"Cookie": tr.RequestHeader().Values("Cookie")}}
	cookie, err := request.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func requestHeader(ctx context.Context, name string, maximum int) string {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	value := strings.TrimSpace(tr.RequestHeader().Get(name))
	if len(value) > maximum {
		value = value[:maximum]
	}
	return value
}

func clientIP(ctx context.Context) string {
	return newAuditEvent(ctx, domain.Principal{}, "", "", "", nil).ClientIP
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func expectedLoginFailure(err error) bool {
	return errors.Is(err, appidentity.ErrInvalidCredentials) || errors.Is(err, appidentity.ErrLocked) || errors.Is(err, appidentity.ErrDisabled) || errors.Is(err, appidentity.ErrMFARequired) || errors.Is(err, appidentity.ErrInvalidMFA)
}
