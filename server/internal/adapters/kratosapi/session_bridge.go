package kratosapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

const (
	bridgeTicketTTL         = 30 * time.Second
	bridgeTicketKeyPrefix   = "auth:session-bridge:"
	gatewaySessionKeyPrefix = "auth:gateway-session:"
	casdoorSessionCookie    = "casdoor_session_id"
	gatewaySessionCookie    = "velora_auth_session"
	bridgeNonceCookie       = "velora_bridge_nonce"
	bridgeNonceParam        = "_velora_bridge_nonce"
	gatewaySessionTTL       = 4 * time.Hour
	bridgeNonceTTL          = 5 * time.Minute
	maxBridgeTicket         = 128
	maxBridgeCookie         = 4096
	maxBridgeReturnPath     = 2048
)

// SessionBridge is a one-time, server-side handoff from the Velora portal to
// the Casdoor host. The ticket contains no credentials and expires quickly.
type SessionBridge struct {
	cache              cache.Cache
	resolveApplication func(context.Context, string, string) (authorizationApplication, error)
	authHost           string
	actionURL          string
	portalURL          *url.URL
	portalOrigin       string
	secure             bool
	sameSite           http.SameSite
	validateSession    func(context.Context, string) (domain.Principal, error)
	authorizeApp       func(context.Context, domain.Principal, string) error
}

type bridgePayload struct {
	Cookie         string `json:"cookie"`
	ReturnPath     string `json:"return_path"`
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	SessionID      string `json:"session_id"`
	NonceDigest    string `json:"nonce_digest,omitempty"`
}

type gatewaySession struct {
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	SessionID      string `json:"session_id"`
}

func NewSessionBridge(c cache.Cache, db *database.DB, accountURL, portalURL string, secure bool, sameSite http.SameSite) (*SessionBridge, error) {
	if c == nil || db == nil {
		return nil, errors.New("session bridge cache and database are required")
	}
	u, err := url.Parse(strings.TrimSpace(accountURL))
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && secure) {
		return nil, errors.New("session bridge account URL must have a valid host")
	}
	actionURL := *u
	actionURL.Path = "/_velora/session/bridge"
	actionURL.RawPath = ""
	actionURL.RawQuery = ""
	actionURL.Fragment = ""
	action := actionURL.String()
	portal, err := url.Parse(strings.TrimSpace(portalURL))
	if err != nil || portal.Hostname() == "" || (portal.Scheme != "https" && secure) || portal.User != nil || portal.RawQuery != "" || portal.Fragment != "" {
		return nil, errors.New("session bridge portal URL must have a valid host")
	}
	portalOrigin := strings.ToLower(portal.Scheme + "://" + portal.Host)
	return &SessionBridge{cache: c, resolveApplication: resolveAuthorizationApplication(db), authHost: strings.ToLower(u.Hostname()), actionURL: action, portalURL: portal, portalOrigin: portalOrigin, secure: secure, sameSite: sameSite}, nil
}

func (b *SessionBridge) ConfigureAccessControl(validateSession func(context.Context, string) (domain.Principal, error), authorizeApp func(context.Context, domain.Principal, string) error) {
	b.validateSession = validateSession
	b.authorizeApp = authorizeApp
}

func (b *SessionBridge) ActionURL() string {
	if b == nil {
		return ""
	}
	return b.actionURL
}

func (b *SessionBridge) Create(ctx context.Context, cookieValue, returnPath string, principal domain.Principal) (string, error) {
	if b == nil || b.cache == nil || strings.TrimSpace(cookieValue) == "" || len(cookieValue) > maxBridgeCookie ||
		strings.TrimSpace(principal.UserID) == "" || strings.TrimSpace(principal.OrganizationID) == "" || strings.TrimSpace(principal.SessionID) == "" {
		return "", errors.New("session bridge payload is invalid")
	}
	ticket, err := cache.RandomToken(32)
	if err != nil {
		return "", err
	}
	cleanReturnPath, nonceDigest := bridgeBinding(returnPath)
	payload, err := json.Marshal(bridgePayload{Cookie: cookieValue, ReturnPath: cleanReturnPath, UserID: principal.UserID, OrganizationID: principal.OrganizationID, SessionID: principal.SessionID, NonceDigest: nonceDigest})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(ticket))
	if err := b.cache.Set(ctx, bridgeTicketKey(hex.EncodeToString(digest[:])), string(payload), bridgeTicketTTL); err != nil {
		return "", err
	}
	return ticket, nil
}

// Handler is mounted only on the auth host by the edge proxy. It rejects
// query-string tickets so browser history and access logs cannot capture them.
func (b *SessionBridge) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodPost || r.URL.RawQuery != "" || !b.allowedHost(r.Host) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if !b.allowedBridgeOrigin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.ContentLength > 8192 {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		ticket := strings.TrimSpace(r.PostForm.Get("ticket"))
		if len(ticket) == 0 || len(ticket) > maxBridgeTicket {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		digest := sha256.Sum256([]byte(ticket))
		key := bridgeTicketKey(hex.EncodeToString(digest[:]))
		payload, err := b.cache.Get(r.Context(), key)
		if err != nil {
			http.Error(w, "ticket expired", http.StatusGone)
			return
		}
		var handoff bridgePayload
		if json.Unmarshal([]byte(payload), &handoff) != nil || strings.TrimSpace(handoff.Cookie) == "" || len(handoff.Cookie) > maxBridgeCookie ||
			strings.TrimSpace(handoff.UserID) == "" || strings.TrimSpace(handoff.OrganizationID) == "" || strings.TrimSpace(handoff.SessionID) == "" {
			http.Error(w, "invalid ticket", http.StatusGone)
			return
		}
		if handoff.NonceDigest != "" && !validBridgeNonce(r, handoff.NonceDigest) {
			http.Error(w, "browser binding failed", http.StatusForbidden)
			return
		}
		consumed, err := b.cache.CompareAndDelete(r.Context(), key, payload)
		if err != nil || !consumed {
			http.Error(w, "ticket expired", http.StatusGone)
			return
		}
		gatewayToken, err := cache.RandomToken(32)
		gatewayValue, marshalErr := json.Marshal(gatewaySession{UserID: handoff.UserID, OrganizationID: handoff.OrganizationID, SessionID: handoff.SessionID})
		if err != nil || marshalErr != nil || b.cache.Set(r.Context(), gatewaySessionKey(gatewayToken), string(gatewayValue), gatewaySessionTTL) != nil {
			http.Error(w, "session unavailable", http.StatusServiceUnavailable)
			return
		}
		// Domain is intentionally omitted: this is a host-only Casdoor cookie.
		// #nosec G124 -- production configuration rejects insecure cookies; local HTTP is an explicit development-only mode.
		http.SetCookie(w, &http.Cookie{Name: casdoorSessionCookie, Value: handoff.Cookie, Path: "/", HttpOnly: true, Secure: b.secure, SameSite: b.sameSite})
		// #nosec G124 -- production configuration rejects insecure cookies; local HTTP is an explicit development-only mode.
		http.SetCookie(w, &http.Cookie{Name: gatewaySessionCookie, Value: gatewayToken, Path: "/", HttpOnly: true, Secure: b.secure, SameSite: b.sameSite, MaxAge: int(gatewaySessionTTL.Seconds())})
		if handoff.NonceDigest != "" {
			b.clearBridgeNonceCookie(w)
		}
		http.Redirect(w, r, b.returnURL(handoff.ReturnPath), http.StatusSeeOther)
	})
}

func (b *SessionBridge) allowedBridgeOrigin(r *http.Request) bool {
	origin := strings.ToLower(strings.TrimSpace(r.Header.Get("Origin")))
	if origin == "" || origin != b.portalOrigin {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site")
}

func (b *SessionBridge) returnURL(returnPath string) string {
	relative, err := url.Parse(safeBridgeReturnPath(returnPath))
	if err != nil {
		relative = &url.URL{Path: "/"}
	}
	target := *b.portalURL
	if relative.Path == "/login/oauth/authorize" {
		target.Scheme = "https"
		target.Host = b.authHost
	}
	target.Path = relative.Path
	target.RawPath = relative.RawPath
	target.RawQuery = relative.RawQuery
	target.Fragment = ""
	return target.String()
}

func (b *SessionBridge) allowedHost(raw string) bool {
	host := strings.ToLower(strings.TrimSpace(raw))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host != "" && host == b.authHost
}

func bridgeTicketKey(digest string) string { return bridgeTicketKeyPrefix + digest }

func gatewaySessionKey(token string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return gatewaySessionKeyPrefix + hex.EncodeToString(digest[:])
}

func safeBridgeReturnPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxBridgeReturnPath || strings.ContainsAny(value, "\r\n\\") || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.Fragment != "" {
		return "/"
	}
	return value
}

func bridgeBinding(returnPath string) (string, string) {
	clean := safeBridgeReturnPath(returnPath)
	u, err := url.Parse(clean)
	if err != nil {
		return "/", ""
	}
	query := u.Query()
	nonce := strings.TrimSpace(query.Get(bridgeNonceParam))
	query.Del(bridgeNonceParam)
	u.RawQuery = query.Encode()
	if nonce == "" || len(nonce) > maxBridgeTicket {
		return u.String(), ""
	}
	digest := sha256.Sum256([]byte(nonce))
	return u.String(), hex.EncodeToString(digest[:])
}

func validBridgeNonce(r *http.Request, want string) bool {
	cookie, err := r.Cookie(bridgeNonceCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" || len(cookie.Value) > maxBridgeTicket {
		return false
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	got := hex.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (b *SessionBridge) setBridgeNonceCookie(w http.ResponseWriter, nonce string) {
	// #nosec G124 -- production configuration requires Secure; local HTTP remains an explicit development-only mode.
	http.SetCookie(w, &http.Cookie{Name: bridgeNonceCookie, Value: nonce, Path: "/", HttpOnly: true, Secure: b.secure, SameSite: b.sameSite, MaxAge: int(bridgeNonceTTL.Seconds())})
}

func (b *SessionBridge) clearBridgeNonceCookie(w http.ResponseWriter) {
	// #nosec G124 -- deletion mirrors the dynamically secured browser-binding cookie.
	http.SetCookie(w, &http.Cookie{Name: bridgeNonceCookie, Value: "", Path: "/", HttpOnly: true, Secure: b.secure, SameSite: b.sameSite, MaxAge: -1})
}
