package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	transactionCookie = "__Host-demo_oidc_tx"
	sessionCookie     = "__Host-demo_session"
	transactionTTL    = 5 * time.Minute
	sessionTTL        = 8 * time.Hour
)

type transaction struct {
	State     string
	Nonce     string
	Verifier  string
	ExpiresAt time.Time
}

type session struct {
	Subject   string
	Name      string
	Email     string
	ExpiresAt time.Time
	IDToken   string
}

type demo struct {
	issuer        string
	publicURL     string
	redirectURL   string
	logoutURL     string
	client        oauth2.Config
	provider      *oidc.Provider
	verifier      *oidc.IDTokenVerifier
	endSessionURL string
	secure        bool
	mu            sync.Mutex
	tx            map[string]transaction
	sessions      map[string]session
}

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Velora OIDC Demo</title>
<style>body{font:16px system-ui,sans-serif;max-width:720px;margin:10vh auto;padding:0 24px;color:#172033}main{border:1px solid #d9e1ec;border-radius:16px;padding:32px;box-shadow:0 12px 32px #17203312}a,button{display:inline-block;padding:10px 16px;border-radius:8px;border:1px solid #1677ff;background:#1677ff;color:white;text-decoration:none;cursor:pointer}small{color:#667085}dt{font-weight:600;margin-top:12px}dd{margin:4px 0 0;word-break:break-all}</style></head>
<body><main><h1>Velora OIDC Demo</h1>{{if .Session}}<p>已通过 Casdoor OIDC 完成登录。</p><dl><dt>Subject</dt><dd>{{.Session.Subject}}</dd><dt>姓名</dt><dd>{{.Session.Name}}</dd><dt>邮箱</dt><dd>{{.Session.Email}}</dd></dl><p><a href="/logout">退出登录</a></p>{{else}}<p>这是用于验收 Authorization Code + PKCE、State、Nonce 和 Token 校验的参考应用。</p><p><a href="/login">使用 Velora 统一身份登录</a></p>{{end}}<p><small>Issuer: {{.Issuer}}</small></p></main></body></html>`))

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	d, err := newDemo(ctx)
	if err != nil {
		slog.Error("oidc demo configuration failed", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: env("DEMO_LISTEN_ADDR", ":8090"), Handler: d.routes(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("oidc demo starting", "addr", server.Addr, "issuer", d.issuer)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("oidc demo stopped", "error", err)
		os.Exit(1)
	}
}

func newDemo(ctx context.Context) (*demo, error) {
	issuer := strings.TrimRight(strings.TrimSpace(os.Getenv("DEMO_OIDC_ISSUER")), "/")
	clientID := strings.TrimSpace(os.Getenv("DEMO_OIDC_CLIENT_ID"))
	redirectURL := strings.TrimSpace(os.Getenv("DEMO_OIDC_REDIRECT_URL"))
	if issuer == "" || clientID == "" || redirectURL == "" {
		return nil, errors.New("DEMO_OIDC_ISSUER, DEMO_OIDC_CLIENT_ID and DEMO_OIDC_REDIRECT_URL are required")
	}
	if !strings.HasPrefix(issuer, "https://") || !strings.HasPrefix(redirectURL, "https://") {
		return nil, errors.New("demo OIDC issuer and redirect must use HTTPS")
	}
	secret, err := readSecret("DEMO_OIDC_CLIENT_SECRET")
	if err != nil {
		return nil, err
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", err)
	}
	var metadata struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("OIDC metadata: %w", err)
	}
	publicURL := strings.TrimRight(strings.TrimSpace(os.Getenv("DEMO_PUBLIC_URL")), "/")
	logoutURL := strings.TrimSpace(os.Getenv("DEMO_POST_LOGOUT_REDIRECT_URL"))
	if logoutURL == "" {
		logoutURL = publicURL + "/"
	}
	if publicURL == "" || !strings.HasPrefix(publicURL, "https://") || !strings.HasPrefix(logoutURL, "https://") {
		return nil, errors.New("DEMO_PUBLIC_URL and DEMO_POST_LOGOUT_REDIRECT_URL must use HTTPS")
	}
	return &demo{issuer: issuer, publicURL: publicURL, redirectURL: redirectURL, logoutURL: logoutURL, client: oauth2.Config{ClientID: clientID, ClientSecret: secret, Endpoint: provider.Endpoint(), RedirectURL: redirectURL, Scopes: []string{oidc.ScopeOpenID, "profile", "email"}}, provider: provider, verifier: provider.Verifier(&oidc.Config{ClientID: clientID}), endSessionURL: strings.TrimSpace(metadata.EndSessionEndpoint), secure: true, tx: make(map[string]transaction), sessions: make(map[string]session)}, nil
}

func (d *demo) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ready\n")
	})
	mux.HandleFunc("/", d.index)
	mux.HandleFunc("/login", d.login)
	mux.HandleFunc("/oauth/callback", d.callback)
	mux.HandleFunc("/logout", d.logout)
	return securityHeaders(mux)
}

func (d *demo) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	d.prune()
	var current *session
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		d.mu.Lock()
		if value, ok := d.sessions[cookie.Value]; ok && time.Now().Before(value.ExpiresAt) {
			copy := value
			current = &copy
		}
		d.mu.Unlock()
	}
	w.Header().Set("Cache-Control", "no-store")
	_ = page.Execute(w, map[string]any{"Session": current, "Issuer": d.issuer})
}

func (d *demo) login(w http.ResponseWriter, r *http.Request) {
	state, err := random(32)
	if err != nil {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	nonce, _ := random(32)
	verifier, _ := random(32)
	d.mu.Lock()
	d.tx[state] = transaction{State: state, Nonce: nonce, Verifier: verifier, ExpiresAt: time.Now().Add(transactionTTL)}
	d.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: transactionCookie, Value: state, Path: "/", HttpOnly: true, Secure: d.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(transactionTTL.Seconds())})
	u := d.client.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce), oauth2.SetAuthURLParam("code_challenge", pkce(verifier)), oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	http.Redirect(w, r, u, http.StatusFound)
}

func (d *demo) callback(w http.ResponseWriter, r *http.Request) {
	state, err := r.Cookie(transactionCookie)
	if err != nil || r.URL.Query().Get("state") == "" || !constantEqual(state.Value, r.URL.Query().Get("state")) {
		http.Error(w, "invalid login transaction", http.StatusUnauthorized)
		return
	}
	d.mu.Lock()
	tx, ok := d.tx[state.Value]
	delete(d.tx, state.Value)
	d.mu.Unlock()
	if !ok || time.Now().After(tx.ExpiresAt) || r.URL.Query().Get("error") != "" {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}
	token, err := d.client.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.SetAuthURLParam("code_verifier", tx.Verifier))
	if err != nil {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}
	idToken, err := d.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}
	var claims struct{ Subject, Name, Email, Nonce string }
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Nonce != tx.Nonce {
		http.Error(w, "login failed", http.StatusUnauthorized)
		return
	}
	sessionID, err := random(32)
	if err != nil {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	d.mu.Lock()
	d.sessions[sessionID] = session{Subject: claims.Subject, Name: claims.Name, Email: claims.Email, IDToken: rawIDToken, ExpiresAt: time.Now().Add(sessionTTL)}
	d.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: transactionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: d.secure, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: sessionID, Path: "/", MaxAge: int(sessionTTL.Seconds()), HttpOnly: true, Secure: d.secure, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (d *demo) logout(w http.ResponseWriter, r *http.Request) {
	var idToken string
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		d.mu.Lock()
		idToken = d.sessions[cookie.Value].IDToken
		delete(d.sessions, cookie.Value)
		d.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: d.secure, SameSite: http.SameSiteLaxMode})
	if d.endSessionURL != "" {
		u, err := url.Parse(d.endSessionURL)
		if err == nil {
			q := u.Query()
			if idToken != "" {
				q.Set("id_token_hint", idToken)
			}
			q.Set("post_logout_redirect_uri", d.logoutURL)
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusFound)
			return
		}
	}
	http.Redirect(w, r, d.logoutURL, http.StatusSeeOther)
}

func (d *demo) prune() {
	now := time.Now()
	d.mu.Lock()
	for key, value := range d.tx {
		if now.After(value.ExpiresAt) {
			delete(d.tx, key)
		}
	}
	for key, value := range d.sessions {
		if now.After(value.ExpiresAt) {
			delete(d.sessions, key)
		}
	}
	d.mu.Unlock()
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func readSecret(name string) (string, error) {
	if path := strings.TrimSpace(os.Getenv(name + "_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s file: %w", name, err)
		}
		if value := strings.TrimSpace(string(data)); value != "" {
			return value, nil
		}
	}
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value, nil
	}
	return "", fmt.Errorf("%s or %s_FILE is required", name, name)
}

func random(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkce(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func constantEqual(a, b string) bool {
	digestA := sha256.Sum256([]byte(a))
	digestB := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(digestA[:], digestB[:]) == 1
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
