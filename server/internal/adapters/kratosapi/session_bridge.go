package kratosapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/transport"
	"github.com/sevoniva-labs/velora/server/internal/platform/cache"
)

const (
	bridgeTicketTTL       = 30 * time.Second
	bridgeTicketKeyPrefix = "auth:session-bridge:"
	casdoorSessionCookie  = "casdoor_session_id"
	maxBridgeTicket       = 128
	maxBridgeCookie       = 4096
	maxBridgeReturnPath   = 2048
)

// SessionBridge is a one-time, server-side handoff from the Velora portal to
// the Casdoor host. The ticket contains no credentials and expires quickly.
type SessionBridge struct {
	cache     cache.Cache
	authHost  string
	actionURL string
	portalURL *url.URL
	secure    bool
	sameSite  http.SameSite
}

type bridgePayload struct {
	Cookie     string `json:"cookie"`
	ReturnPath string `json:"return_path"`
}

func NewSessionBridge(c cache.Cache, accountURL, portalURL string, secure bool, sameSite http.SameSite) (*SessionBridge, error) {
	if c == nil {
		return nil, errors.New("session bridge cache is required")
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
	return &SessionBridge{cache: c, authHost: strings.ToLower(u.Hostname()), actionURL: action, portalURL: portal, secure: secure, sameSite: sameSite}, nil
}

func (b *SessionBridge) ActionURL() string {
	if b == nil {
		return ""
	}
	return b.actionURL
}

func (b *SessionBridge) Create(ctx context.Context, cookieValue, returnPath string) (string, error) {
	if b == nil || b.cache == nil || strings.TrimSpace(cookieValue) == "" || len(cookieValue) > maxBridgeCookie {
		return "", errors.New("session bridge payload is invalid")
	}
	ticket, err := cache.RandomToken(32)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(bridgePayload{Cookie: cookieValue, ReturnPath: safeBridgeReturnPath(returnPath)})
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
		consumed, err := b.cache.CompareAndDelete(r.Context(), key, payload)
		if err != nil || !consumed {
			http.Error(w, "ticket expired", http.StatusGone)
			return
		}
		var handoff bridgePayload
		if json.Unmarshal([]byte(payload), &handoff) != nil || strings.TrimSpace(handoff.Cookie) == "" || len(handoff.Cookie) > maxBridgeCookie {
			http.Error(w, "invalid ticket", http.StatusGone)
			return
		}
		// Domain is intentionally omitted: this is a host-only Casdoor cookie.
		http.SetCookie(w, &http.Cookie{Name: casdoorSessionCookie, Value: handoff.Cookie, Path: "/", HttpOnly: true, Secure: b.secure, SameSite: b.sameSite})
		http.Redirect(w, r, b.returnURL(handoff.ReturnPath), http.StatusSeeOther)
	})
}

func (b *SessionBridge) returnURL(returnPath string) string {
	relative, err := url.Parse(safeBridgeReturnPath(returnPath))
	if err != nil {
		relative = &url.URL{Path: "/"}
	}
	target := *b.portalURL
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

func bridgeRequestCookie(ctx context.Context, name string) string {
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
