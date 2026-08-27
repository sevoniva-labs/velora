package kratosapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
	"github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/identitysource"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	wechatStateCookie    = "velora_wechat_tx"
	wechatStatePrefix    = "auth:wechat:state:"
	wechatBindPrefix     = "auth:wechat:bind:"
	wechatCompletePrefix = "auth:wechat:complete:"
	wechatStateTTL       = 5 * time.Minute
	wechatCompleteTTL    = 60 * time.Second
)

type wechatIdentityManager interface {
	ValidateWeChatPolicy(context.Context, string) error
	WeChatBinding(context.Context, string) (bool, error)
	UnlinkWeChat(context.Context, string) error
}

type WeChatConfig struct {
	Enabled                      bool
	AppID, Provider, CallbackURL string
	Secure                       bool
}

type WeChatBroker struct {
	config          WeChatConfig
	cache           cache.Cache
	db              *database.DB
	provider        *identitysource.OIDCProvider
	bridge          *SessionBridge
	manager         wechatIdentityManager
	identityService *IdentityService
	portal          *url.URL
	authHost        string
	metrics         wechatMetrics
}

type wechatMetrics interface {
	ObserveIdentity(flow, result string, start time.Time)
}

type wechatState struct {
	Mode, ReturnPath, UserID, OrganizationID, LoginName, SessionID string
}

type wechatCompletion struct {
	Subject, CasdoorCookie, ReturnPath string
	MFAVerified                        bool
}

func NewWeChatBroker(cfg WeChatConfig, c cache.Cache, db *database.DB, provider *identitysource.OIDCProvider, bridge *SessionBridge, manager wechatIdentityManager, service *IdentityService, metrics wechatMetrics) (*WeChatBroker, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	callback, err := url.Parse(strings.TrimSpace(cfg.CallbackURL))
	if err != nil || callback.Scheme != "https" || callback.Host == "" || callback.Path != "/_velora/wechat/callback" || callback.RawQuery != "" || callback.Fragment != "" {
		return nil, errors.New("invalid WeChat callback URL")
	}
	if c == nil || c.Provider() == "disabled" || db == nil || provider == nil || bridge == nil || manager == nil || service == nil || strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.Provider) == "" {
		return nil, errors.New("WeChat broker dependencies are incomplete")
	}
	policyCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := manager.ValidateWeChatPolicy(policyCtx, cfg.Provider); err != nil {
		return nil, fmt.Errorf("unsafe Casdoor WeChat policy: %w", err)
	}
	return &WeChatBroker{config: cfg, cache: c, db: db, provider: provider, bridge: bridge, manager: manager, identityService: service, portal: bridge.portalURL, authHost: strings.ToLower(callback.Hostname()), metrics: metrics}, nil
}

func (b *WeChatBroker) observe(flow, result string, start time.Time) {
	if b != nil && b.metrics != nil {
		b.metrics.ObserveIdentity(flow, result, start)
	}
}

func (b *WeChatBroker) StartURL(returnPath string) string {
	if b == nil {
		return ""
	}
	q := url.Values{}
	if clean := safeReturnPath(returnPath); clean != "/" {
		q.Set("return", clean)
	}
	return "https://" + b.authHost + "/_velora/wechat/start" + func() string {
		if len(q) == 0 {
			return ""
		}
		return "?" + q.Encode()
	}()
}

func (b *WeChatBroker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.ToLower(strings.Split(r.Host, ":")[0]) != b.authHost {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/_velora/wechat/start":
			b.start(w, r)
		case "/_velora/wechat/callback":
			b.callback(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func (b *WeChatBroker) start(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	tx := wechatState{Mode: "login", ReturnPath: safeReturnPath(r.URL.Query().Get("return"))}
	if ticket := strings.TrimSpace(r.URL.Query().Get("binding_ticket")); ticket != "" {
		payload, ok := b.consume(r.Context(), wechatBindPrefix, ticket)
		if !ok || json.Unmarshal([]byte(payload), &tx) != nil || tx.Mode != "bind" || tx.UserID == "" {
			b.observe("wechat_start", "invalid_binding_ticket", started)
			b.fail(w, r, "/user-center?wechat=failed")
			return
		}
	}
	state, err := cache.RandomToken(32)
	if err != nil {
		b.observe("wechat_start", "token_failed", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	payload, _ := json.Marshal(tx)
	if b.cache.Set(r.Context(), wechatStatePrefix+digestToken(state), string(payload), wechatStateTTL) != nil {
		b.observe("wechat_start", "state_store_failed", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: wechatStateCookie, Value: state, Path: "/_velora/wechat/", MaxAge: int(wechatStateTTL.Seconds()), HttpOnly: true, Secure: b.config.Secure, SameSite: http.SameSiteLaxMode})
	callback := b.config.CallbackURL
	q := url.Values{"appid": {b.config.AppID}, "redirect_uri": {callback}, "response_type": {"code"}, "scope": {"snsapi_login"}, "state": {state}}
	b.observe("wechat_start", "success", started)
	http.Redirect(w, r, "https://open.weixin.qq.com/connect/qrconnect?"+q.Encode()+"#wechat_redirect", http.StatusFound)
}

func (b *WeChatBroker) callback(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	state, code := strings.TrimSpace(r.URL.Query().Get("state")), strings.TrimSpace(r.URL.Query().Get("code"))
	cookie, err := r.Cookie(wechatStateCookie)
	http.SetCookie(w, &http.Cookie{Name: wechatStateCookie, Path: "/_velora/wechat/", MaxAge: -1, Expires: time.Unix(0, 0), HttpOnly: true, Secure: b.config.Secure, SameSite: http.SameSiteLaxMode})
	if err != nil || state == "" || code == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		b.observe("wechat_callback", "invalid_state", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	payload, ok := b.consume(r.Context(), wechatStatePrefix, state)
	if !ok {
		b.observe("wechat_callback", "replayed_state", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	var tx wechatState
	if json.Unmarshal([]byte(payload), &tx) != nil {
		b.observe("wechat_callback", "invalid_state", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	if tx.Mode == "bind" {
		casdoor, e := r.Cookie(casdoorSessionCookie)
		if e != nil || b.provider.LinkProviderCode(r.Context(), b.config.Provider, code, b.config.CallbackURL, casdoor.Value) != nil {
			b.observe("wechat_bind", "provider_failed", started)
			b.fail(w, r, "/user-center?wechat=failed")
			return
		}
		linked, verifyErr := b.manager.WeChatBinding(r.Context(), tx.LoginName)
		if verifyErr != nil || !linked {
			b.observe("wechat_bind", "verification_failed", started)
			_ = b.manager.UnlinkWeChat(r.Context(), tx.LoginName)
			b.fail(w, r, "/user-center?wechat=failed")
			return
		}
		if _, e = b.db.ExecContext(r.Context(), b.db.Rebind(`DELETE FROM user_wechat_bindings WHERE user_id=?`), tx.UserID); e == nil {
			_, e = b.db.ExecContext(r.Context(), b.db.Rebind(`INSERT INTO user_wechat_bindings(user_id,provider,bound_at,version) VALUES(?,?,?,1)`), tx.UserID, b.config.Provider, time.Now().UTC())
		}
		if e != nil {
			b.observe("wechat_bind", "storage_failed", started)
			_ = b.manager.UnlinkWeChat(r.Context(), tx.LoginName)
			b.fail(w, r, "/user-center?wechat=failed")
			return
		}
		_ = b.identityService.audit.Write(r.Context(), *newAuditEvent(r.Context(), identity.Principal{UserID: tx.UserID, OrganizationID: tx.OrganizationID, LoginName: tx.LoginName, SessionID: tx.SessionID}, "identity.wechat.bind", "user", tx.UserID, nil))
		b.observe("wechat_bind", "success", started)
		b.fail(w, r, "/user-center?wechat=bound")
		return
	}
	fed, err := b.provider.AuthenticateProviderCode(r.Context(), b.config.Provider, code, b.config.CallbackURL)
	if err != nil || fed.Subject == "" {
		b.observe("wechat_callback", "provider_failed", started)
		b.fail(w, r, "/login?wechat=unbound")
		return
	}
	if !b.boundSubject(r.Context(), b.provider.Name(), fed.Subject) {
		b.observe("wechat_callback", "unbound", started)
		b.fail(w, r, "/login?wechat=unbound")
		return
	}
	ticket, err := cache.RandomToken(32)
	if err != nil {
		b.observe("wechat_callback", "token_failed", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	completion, _ := json.Marshal(wechatCompletion{Subject: fed.Subject, MFAVerified: fed.MFAVerified, CasdoorCookie: fed.CasdoorSessionCookie, ReturnPath: tx.ReturnPath})
	if b.cache.Set(r.Context(), wechatCompletePrefix+digestToken(ticket), string(completion), wechatCompleteTTL) != nil {
		b.observe("wechat_callback", "state_store_failed", started)
		b.fail(w, r, "/login?wechat=failed")
		return
	}
	target := "/login?wechat_ticket=" + url.QueryEscape(ticket)
	if tx.ReturnPath != "/" {
		target += "&redirect=" + url.QueryEscape(tx.ReturnPath)
	}
	b.observe("wechat_callback", "success", started)
	b.fail(w, r, target)
}

func (b *WeChatBroker) boundSubject(ctx context.Context, provider, subject string) bool {
	var found int
	err := b.db.QueryRowContext(ctx, b.db.Rebind(`SELECT 1 FROM user_wechat_bindings wb WHERE wb.user_id IN (
SELECT u.id FROM users u JOIN organizations o ON o.id=u.organization_id WHERE o.org_key='default' AND LOWER(u.identity_source)=? AND u.external_subject=?
UNION SELECT e.user_id FROM external_identities e JOIN organizations o ON o.id=e.organization_id WHERE o.org_key='default' AND e.provider=? AND e.subject=?
)`), strings.ToLower(strings.TrimSpace(provider)), subject, strings.ToLower(strings.TrimSpace(provider)), subject).Scan(&found)
	return err == nil && found == 1
}

func (b *WeChatBroker) fail(w http.ResponseWriter, r *http.Request, path string) {
	u := *b.portal
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	target, _ := u.Parse(path)
	http.Redirect(w, r, target.String(), http.StatusFound)
}
func digestToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (b *WeChatBroker) consume(ctx context.Context, prefix, token string) (string, bool) {
	if len(token) < 32 || len(token) > 128 {
		return "", false
	}
	key := prefix + digestToken(token)
	p, e := b.cache.Get(ctx, key)
	if e != nil {
		return "", false
	}
	ok, e := b.cache.CompareAndDelete(ctx, key, p)
	return p, e == nil && ok
}
func safeReturnPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") || len(raw) > 2048 {
		return "/"
	}
	return raw
}

func (s *IdentityService) CompleteWeChatLogin(ctx context.Context, req *forgev1.CompleteWeChatLoginRequest) (*forgev1.LoginResponse, error) {
	started := time.Now()
	if s.wechat == nil {
		return nil, kerrors.ServiceUnavailable("WECHAT_DISABLED", "WeChat login is unavailable")
	}
	ticketValue := strings.TrimSpace(req.GetTicket())
	if len(ticketValue) < 32 || len(ticketValue) > 128 {
		s.wechat.observe("wechat_complete", "invalid_ticket", started)
		return nil, kerrors.Unauthorized("WECHAT_LOGIN_FAILED", "WeChat login transaction is invalid")
	}
	key := wechatCompletePrefix + digestToken(ticketValue)
	payload, err := s.wechat.cache.Get(ctx, key)
	if err != nil {
		s.wechat.observe("wechat_complete", "invalid_ticket", started)
		return nil, kerrors.Unauthorized("WECHAT_LOGIN_FAILED", "WeChat login transaction is invalid")
	}
	var item wechatCompletion
	if json.Unmarshal([]byte(payload), &item) != nil || item.Subject == "" || item.CasdoorCookie == "" {
		s.wechat.observe("wechat_complete", "invalid_ticket", started)
		return nil, kerrors.Unauthorized("WECHAT_LOGIN_FAILED", "WeChat login transaction is invalid")
	}
	principal, session, csrf, expires, err := s.loginFederated(ctx, "default", s.wechat.provider.Name(), item.Subject, item.MFAVerified, req.GetMfaCode(), req.GetRecoveryCode())
	if err != nil {
		result := "authentication_failed"
		if kerrors.FromError(err).Code == http.StatusPreconditionRequired {
			result = "mfa_required"
		}
		s.wechat.observe("wechat_complete", result, started)
		return nil, err
	}
	consumed, err := s.wechat.cache.CompareAndDelete(ctx, key, payload)
	if err != nil || !consumed {
		s.wechat.observe("wechat_complete", "replayed_ticket", started)
		return nil, kerrors.Unauthorized("WECHAT_LOGIN_FAILED", "WeChat login transaction was already used")
	}
	_, _ = s.db.ExecContext(ctx, s.db.Rebind(`UPDATE user_wechat_bindings SET last_login_at=?,version=version+1 WHERE user_id=?`), time.Now().UTC(), principal.UserID)
	s.setLoginCookies(ctx, session, csrf, expires)
	res := &forgev1.LoginResponse{User: principalUser(principal), CsrfToken: csrf}
	if s.sessionBridge != nil {
		ticket, err := s.sessionBridge.Create(ctx, item.CasdoorCookie, safeReturnPath(req.GetReturnPath()), principal)
		if err != nil {
			s.wechat.observe("wechat_complete", "bridge_failed", started)
			return nil, federatedUnavailable()
		}
		res.BridgeAction = s.sessionBridge.ActionURL()
		res.BridgeTicket = ticket
	}
	s.wechat.observe("wechat_complete", "success", started)
	return res, nil
}

func (s *IdentityService) GetWeChatBinding(ctx context.Context, _ *forgev1.GetWeChatBindingRequest) (*forgev1.GetWeChatBindingResponse, error) {
	principal, ok := authn.Principal(ctx)
	if !ok {
		return nil, kerrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	res := &forgev1.GetWeChatBindingResponse{Enabled: s.wechat != nil}
	if s.wechat == nil {
		return res, nil
	}
	upstreamBound, upstreamErr := s.wechat.manager.WeChatBinding(ctx, principal.LoginName)
	if upstreamErr != nil {
		return nil, kerrors.ServiceUnavailable("WECHAT_STATUS_UNAVAILABLE", "WeChat binding status is unavailable")
	}
	var at time.Time
	err := s.db.QueryRowContext(ctx, s.db.Rebind(`SELECT bound_at FROM user_wechat_bindings WHERE user_id=?`), principal.UserID).Scan(&at)
	if !upstreamBound {
		if err == nil {
			_, _ = s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM user_wechat_bindings WHERE user_id=?`), principal.UserID)
		}
		return res, nil
	}
	if err != nil {
		at = time.Now().UTC()
		if _, insertErr := s.db.ExecContext(ctx, s.db.Rebind(`INSERT INTO user_wechat_bindings(user_id,provider,bound_at,version) VALUES(?,?,?,1)`), principal.UserID, s.wechat.config.Provider, at); insertErr != nil {
			return nil, internalError(insertErr)
		}
	}
	if upstreamBound {
		res.Bound = true
		res.BoundAt = timestamppb.New(at)
	}
	return res, nil
}

func (s *IdentityService) BeginWeChatBinding(ctx context.Context, _ *forgev1.BeginWeChatBindingRequest) (*forgev1.BeginWeChatBindingResponse, error) {
	principal, ok := authn.Principal(ctx)
	if !ok {
		return nil, kerrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	if s.wechat == nil {
		return nil, kerrors.ServiceUnavailable("WECHAT_DISABLED", "WeChat binding is unavailable")
	}
	if enabled, err := s.identity.MFAEnabled(ctx, principal); err != nil {
		return nil, internalError(err)
	} else if enabled {
		if err = appidentity.RequireRecentMFA(principal); err != nil {
			return nil, kerrors.Forbidden("STEP_UP_REQUIRED", "recent multi-factor authentication is required")
		}
	}
	if allowed, err := s.limiter.Allow(ctx, "wechat-bind|"+principal.UserID, 5, time.Hour, time.Now()); err != nil || !allowed {
		return nil, kerrors.New(http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
	}
	ticket, err := cache.RandomToken(32)
	if err != nil {
		return nil, federatedUnavailable()
	}
	tx := wechatState{Mode: "bind", UserID: principal.UserID, OrganizationID: principal.OrganizationID, LoginName: principal.LoginName, SessionID: principal.SessionID}
	payload, _ := json.Marshal(tx)
	if s.wechat.cache.Set(ctx, wechatBindPrefix+digestToken(ticket), string(payload), wechatStateTTL) != nil {
		return nil, federatedUnavailable()
	}
	return &forgev1.BeginWeChatBindingResponse{RedirectUrl: "https://" + s.wechat.authHost + "/_velora/wechat/start?binding_ticket=" + url.QueryEscape(ticket)}, nil
}

func (s *IdentityService) DeleteWeChatBinding(ctx context.Context, _ *forgev1.DeleteWeChatBindingRequest) (*forgev1.DeleteWeChatBindingResponse, error) {
	principal, ok := authn.Principal(ctx)
	if !ok {
		return nil, kerrors.Unauthorized("UNAUTHENTICATED", "authentication required")
	}
	if s.wechat == nil {
		return nil, kerrors.ServiceUnavailable("WECHAT_DISABLED", "WeChat binding is unavailable")
	}
	if enabled, err := s.identity.MFAEnabled(ctx, principal); err != nil {
		return nil, internalError(err)
	} else if enabled {
		if err = appidentity.RequireRecentMFA(principal); err != nil {
			return nil, kerrors.Forbidden("STEP_UP_REQUIRED", "recent multi-factor authentication is required")
		}
	}
	if err := s.wechat.manager.UnlinkWeChat(ctx, principal.LoginName); err != nil {
		return nil, kerrors.ServiceUnavailable("WECHAT_UNLINK_FAILED", "WeChat account could not be unlinked")
	}
	if _, err := s.db.ExecContext(ctx, s.db.Rebind(`DELETE FROM user_wechat_bindings WHERE user_id=?`), principal.UserID); err != nil {
		return nil, internalError(err)
	}
	_ = s.audit.Write(ctx, *newAuditEvent(ctx, principal, "identity.wechat.unbind", "user", principal.UserID, nil))
	return &forgev1.DeleteWeChatBindingResponse{}, nil
}
