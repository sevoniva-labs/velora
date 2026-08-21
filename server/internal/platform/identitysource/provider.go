package identitysource

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-ldap/ldap/v3"
	"golang.org/x/oauth2"
)

var (
	ErrInvalidConfiguration = errors.New("invalid identity source configuration")
	ErrAuthenticationFailed = errors.New("federated authentication failed")
)

type FederatedIdentity struct {
	Subject     string
	LoginName   string
	DisplayName string
	Email       string
	Groups      []string
	MFAVerified bool
	Provider    string
	// CasdoorSessionCookie is captured only by the explicit password bridge
	// flow. It is never logged or persisted outside the one-time bridge ticket.
	CasdoorSessionCookie string
	AuthenticationAt     time.Time
}

type OIDCConfig struct {
	Name   string
	Issuer string
	// InternalURL optionally rewrites server-side calls to Issuer to an
	// in-network address while keeping the browser-visible issuer unchanged.
	InternalURL           string
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	PostLogoutRedirectURL string
	Scopes                []string
	AllowHTTP             bool
	PasswordLoginEnabled  bool
	Application           string
	Organization          string
}

type OIDCProvider struct {
	name                  string
	config                oauth2.Config
	provider              *oidc.Provider
	verifier              *oidc.IDTokenVerifier
	endSessionEndpoint    string
	postLogoutRedirectURL string
	httpClient            *http.Client
	tokenURL              string
	issuer                string
	passwordLoginEnabled  bool
	application           string
	organization          string
}

func NewOIDCProvider(ctx context.Context, client *http.Client, cfg OIDCConfig) (*OIDCProvider, error) {
	issuer := strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	if issuer == "" || strings.TrimSpace(cfg.Name) == "" || cfg.ClientID == "" || strings.TrimSpace(cfg.ClientSecret) == "" || cfg.RedirectURL == "" {
		return nil, ErrInvalidConfiguration
	}
	if err := validateEndpoint(issuer, cfg.AllowHTTP); err != nil {
		return nil, err
	}
	if err := validateEndpoint(cfg.RedirectURL, cfg.AllowHTTP); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.PostLogoutRedirectURL) != "" {
		if err := validateEndpoint(cfg.PostLogoutRedirectURL, cfg.AllowHTTP); err != nil {
			return nil, err
		}
	}
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(cfg.InternalURL) != "" {
		internal, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.InternalURL), "/"))
		if err != nil || internal.Host == "" || (internal.Scheme != "http" && internal.Scheme != "https") {
			return nil, ErrInvalidConfiguration
		}
		external, _ := url.Parse(issuer)
		transport := client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		clone := *client
		clone.Transport = &internalURLRoundTripper{base: transport, external: external, internal: internal}
		client = &clone
	}
	ctx = oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	var metadata struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("oidc discovery metadata: %w", err)
	}
	if strings.TrimSpace(metadata.EndSessionEndpoint) != "" {
		if err := validateEndpoint(metadata.EndSessionEndpoint, cfg.AllowHTTP); err != nil {
			return nil, fmt.Errorf("oidc end-session endpoint: %w", err)
		}
	}
	scopes := append([]string{"openid", "profile", "email"}, cfg.Scopes...)
	return &OIDCProvider{
		name:                  cfg.Name,
		config:                oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: cfg.RedirectURL, Scopes: uniqueStrings(scopes)},
		provider:              provider,
		verifier:              provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		endSessionEndpoint:    strings.TrimSpace(metadata.EndSessionEndpoint),
		postLogoutRedirectURL: strings.TrimSpace(cfg.PostLogoutRedirectURL),
		httpClient:            client,
		tokenURL:              provider.Endpoint().TokenURL,
		issuer:                issuer,
		passwordLoginEnabled:  cfg.PasswordLoginEnabled,
		application:           strings.TrimSpace(cfg.Application),
		organization:          strings.TrimSpace(cfg.Organization),
	}, nil
}

func (p *OIDCProvider) Name() string { return p.name }

// EndSessionURL returns the provider's standard RP-initiated logout URL. An
// empty result means the discovery document does not advertise end_session.
// Local Velora session revocation must happen independently of this optional
// browser redirect.
func (p *OIDCProvider) EndSessionURL() (string, error) {
	if strings.TrimSpace(p.endSessionEndpoint) == "" {
		return "", nil
	}
	u, err := url.Parse(p.endSessionEndpoint)
	if err != nil || u.Host == "" {
		return "", ErrInvalidConfiguration
	}
	query := u.Query()
	query.Set("client_id", p.config.ClientID)
	if p.postLogoutRedirectURL != "" {
		query.Set("post_logout_redirect_uri", p.postLogoutRedirectURL)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (p *OIDCProvider) AuthorizationURL(state, nonce string, codeChallenge ...string) string {
	options := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("nonce", nonce)}
	if len(codeChallenge) > 0 && strings.TrimSpace(codeChallenge[0]) != "" {
		options = append(options,
			oauth2.SetAuthURLParam("code_challenge", codeChallenge[0]),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		)
	}
	return p.config.AuthCodeURL(state, options...)
}

func (p *OIDCProvider) AuthenticateCode(ctx context.Context, code, nonce string, codeVerifier ...string) (FederatedIdentity, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(nonce) == "" {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	var options []oauth2.AuthCodeOption
	if len(codeVerifier) > 0 && strings.TrimSpace(codeVerifier[0]) != "" {
		options = append(options, oauth2.SetAuthURLParam("code_verifier", codeVerifier[0]))
	}
	ctx = oidc.ClientContext(ctx, p.httpClient)
	token, err := p.config.Exchange(ctx, code, options...)
	// Casdoor's token endpoint accepts the authorization-code exchange as a
	// JSON document, while golang.org/x/oauth2 first sends HTTP Basic/form
	// credentials. Keep the standard exchange as the default for providers,
	// then retry only the provider's explicit invalid_client response using its
	// documented JSON shape. The authorization code is not consumed on an
	// invalid-client response.
	if err != nil && strings.Contains(err.Error(), `"invalid_client"`) {
		var verifier string
		if len(codeVerifier) > 0 {
			verifier = strings.TrimSpace(codeVerifier[0])
		}
		token, err = p.exchangeJSON(ctx, code, verifier)
	}
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	var claims struct {
		Subject           string          `json:"sub"`
		Email             string          `json:"email"`
		Name              string          `json:"name"`
		PreferredUsername string          `json:"preferred_username"`
		Groups            []string        `json:"groups"`
		AMR               json.RawMessage `json:"amr"`
		ACR               string          `json:"acr"`
		Nonce             string          `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Nonce != nonce {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	identity := FederatedIdentity{Subject: claims.Subject, LoginName: claims.PreferredUsername, DisplayName: claims.Name, Email: claims.Email, Groups: uniqueStrings(claims.Groups), MFAVerified: claimsIndicateMFA(claims.AMR, claims.ACR), Provider: p.name, AuthenticationAt: time.Now().UTC()}
	if identity.LoginName == "" {
		identity.LoginName = claims.Email
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.LoginName
	}
	return identity, nil
}

// AuthenticatePassword is a Casdoor-specific compatibility flow for
// deployments that require the Velora page to own the credential form. It
// submits credentials only over the server-to-Casdoor connection, never logs
// them, and immediately exchanges the one-time authorization code for a
// signed ID token which still goes through the normal OIDC verification path.
// Authorization Code + PKCE remains the preferred flow.
func (p *OIDCProvider) AuthenticatePassword(ctx context.Context, loginName, password, mfaCode, recoveryCode string) (FederatedIdentity, error) {
	if !p.passwordLoginEnabled || p.application == "" || p.organization == "" || strings.TrimSpace(loginName) == "" || password == "" {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	if len(loginName) > 120 || len(password) > 512 || len(mfaCode) > 32 || len(recoveryCode) > 128 {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	nonce, err := randomValue(32)
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	state, err := randomValue(32)
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	verifier, err := randomValue(32)
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	endpoint, err := url.Parse(strings.TrimRight(p.issuer, "/") + "/api/login")
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	query := endpoint.Query()
	query.Set("clientId", p.config.ClientID)
	query.Set("responseType", "code")
	query.Set("redirectUri", p.config.RedirectURL)
	query.Set("type", "code")
	query.Set("scope", strings.Join(p.config.Scopes, " "))
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge_method", "S256")
	query.Set("code_challenge", sha256Base64URL(verifier))
	endpoint.RawQuery = query.Encode()

	payload := map[string]string{
		"application":  p.application,
		"organization": p.organization,
		"username":     strings.TrimSpace(loginName),
		"password":     password,
		"type":         "code",
		"signinMethod": "Password",
	}
	if strings.TrimSpace(mfaCode) != "" {
		payload["passcode"] = strings.TrimSpace(mfaCode)
		payload["mfaType"] = "totp"
	}
	if strings.TrimSpace(recoveryCode) != "" {
		payload["recoveryCode"] = strings.TrimSpace(recoveryCode)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	defer resp.Body.Close()
	var result struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !strings.EqualFold(result.Status, "ok") {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	var code string
	if err := json.Unmarshal(result.Data, &code); err != nil || strings.TrimSpace(code) == "" {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	identity, err := p.AuthenticateCode(ctx, code, nonce, verifier)
	if err != nil {
		return FederatedIdentity{}, err
	}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "casdoor_session_id" && strings.TrimSpace(cookie.Value) != "" {
			identity.CasdoorSessionCookie = cookie.Value
			break
		}
	}
	identity.MFAVerified = strings.TrimSpace(mfaCode) != "" || strings.TrimSpace(recoveryCode) != ""
	return identity, nil
}

func claimsIndicateMFA(raw json.RawMessage, acr string) bool {
	if strings.Contains(strings.ToLower(acr), "mfa") || strings.Contains(strings.ToLower(acr), "multi") {
		return true
	}
	var values []string
	if json.Unmarshal(raw, &values) == nil {
		for _, value := range values {
			if strings.EqualFold(strings.TrimSpace(value), "mfa") || strings.EqualFold(strings.TrimSpace(value), "otp") || strings.EqualFold(strings.TrimSpace(value), "totp") || strings.EqualFold(strings.TrimSpace(value), "hwk") {
				return true
			}
		}
		return false
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.EqualFold(strings.TrimSpace(value), "mfa") || strings.EqualFold(strings.TrimSpace(value), "otp") || strings.EqualFold(strings.TrimSpace(value), "totp") || strings.EqualFold(strings.TrimSpace(value), "hwk")
	}
	return false
}

func randomValue(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func sha256Base64URL(value string) string {
	sum := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type internalURLRoundTripper struct {
	base     http.RoundTripper
	external *url.URL
	internal *url.URL
}

func (t *internalURLRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil || t.base == nil || t.external == nil || t.internal == nil {
		return t.base.RoundTrip(req)
	}
	if req.URL.Scheme == t.external.Scheme && req.URL.Host == t.external.Host {
		clone := req.Clone(req.Context())
		*clone.URL = *req.URL
		clone.URL.Scheme = t.internal.Scheme
		clone.URL.Host = t.internal.Host
		if t.internal.Path != "" && t.internal.Path != "/" {
			clone.URL.Path = strings.TrimRight(t.internal.Path, "/") + "/" + strings.TrimLeft(clone.URL.Path, "/")
		}
		return t.base.RoundTrip(clone)
	}
	return t.base.RoundTrip(req)
}

type oidcJSONTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

func (p *OIDCProvider) exchangeJSON(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     p.config.ClientID,
		"client_secret": p.config.ClientSecret,
		"code":          code,
		"redirect_uri":  p.config.RedirectURL,
	}
	if verifier != "" {
		payload["code_verifier"] = verifier
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var decoded oidcJSONTokenResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("oidc token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || decoded.Error != "" || decoded.AccessToken == "" {
		if decoded.Error == "" {
			decoded.Error = resp.Status
		}
		return nil, fmt.Errorf("oidc token exchange: %s: %s", decoded.Error, decoded.Description)
	}
	token := &oauth2.Token{AccessToken: decoded.AccessToken, TokenType: decoded.TokenType, RefreshToken: decoded.RefreshToken}
	if decoded.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(decoded.ExpiresIn) * time.Second)
	}
	if decoded.IDToken != "" {
		token = token.WithExtra(map[string]any{"id_token": decoded.IDToken})
	}
	return token, nil
}

type LDAPConfig struct {
	Name             string
	URL              string
	BindDN           string
	BindPassword     string
	BaseDN           string
	UserFilter       string
	LoginAttribute   string
	DisplayAttribute string
	EmailAttribute   string
	GroupAttribute   string
	StartTLS         bool
	AllowInsecure    bool
	TLSConfig        *tls.Config
	SearchTimeout    time.Duration
}

type LDAPProvider struct {
	name string
	cfg  LDAPConfig
}

func NewLDAPProvider(cfg LDAPConfig) (*LDAPProvider, error) {
	if strings.TrimSpace(cfg.Name) == "" || strings.TrimSpace(cfg.URL) == "" || strings.TrimSpace(cfg.BindDN) == "" || cfg.BaseDN == "" || cfg.LoginAttribute == "" {
		return nil, ErrInvalidConfiguration
	}
	u, err := url.Parse(cfg.URL)
	validLDAPEndpoint := u.Scheme == "ldaps" || (u.Scheme == "ldap" && (cfg.StartTLS || cfg.AllowInsecure))
	if err != nil || u.Host == "" || !validLDAPEndpoint {
		return nil, ErrInvalidConfiguration
	}
	if cfg.SearchTimeout <= 0 {
		cfg.SearchTimeout = 5 * time.Second
	}
	if cfg.UserFilter == "" {
		cfg.UserFilter = "(&(objectClass=person)(%s=%s))"
	}
	if cfg.DisplayAttribute == "" {
		cfg.DisplayAttribute = "displayName"
	}
	if cfg.EmailAttribute == "" {
		cfg.EmailAttribute = "mail"
	}
	if cfg.GroupAttribute == "" {
		cfg.GroupAttribute = "memberOf"
	}
	return &LDAPProvider{name: cfg.Name, cfg: cfg}, nil
}

func (p *LDAPProvider) Name() string { return p.name }

func (p *LDAPProvider) Authenticate(ctx context.Context, login, password string) (FederatedIdentity, error) {
	if strings.TrimSpace(login) == "" || password == "" {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	conn, err := ldap.DialURL(p.cfg.URL, ldap.DialWithTLSConfig(p.cfg.TLSConfig))
	if err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	defer func() { _ = conn.Close() }()
	conn.SetTimeout(p.cfg.SearchTimeout)
	if p.cfg.StartTLS {
		if err := conn.StartTLS(p.cfg.TLSConfig); err != nil {
			return FederatedIdentity{}, ErrAuthenticationFailed
		}
	}
	if err := conn.Bind(p.cfg.BindDN, p.cfg.BindPassword); err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	filter := fmt.Sprintf(p.cfg.UserFilter, ldap.EscapeFilter(p.cfg.LoginAttribute), ldap.EscapeFilter(login))
	attrs := uniqueStrings([]string{p.cfg.LoginAttribute, p.cfg.DisplayAttribute, p.cfg.EmailAttribute, p.cfg.GroupAttribute})
	search := ldap.NewSearchRequest(p.cfg.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, int(p.cfg.SearchTimeout/time.Second), false, filter, attrs, nil)
	result, err := conn.Search(search)
	if err != nil || len(result.Entries) != 1 {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	entry := result.Entries[0]
	if err := conn.Bind(entry.DN, password); err != nil {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	identity := FederatedIdentity{Subject: entry.DN, LoginName: entry.GetAttributeValue(p.cfg.LoginAttribute), DisplayName: entry.GetAttributeValue(p.cfg.DisplayAttribute), Email: entry.GetAttributeValue(p.cfg.EmailAttribute), Groups: uniqueStrings(entry.GetAttributeValues(p.cfg.GroupAttribute)), Provider: p.name, AuthenticationAt: time.Now().UTC()}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.LoginName
	}
	return identity, nil
}

func validateEndpoint(value string, allowHTTP bool) error {
	u, err := url.Parse(strings.TrimSpace(value))
	validHTTPSEndpoint := u.Scheme == "https" || (allowHTTP && u.Scheme == "http")
	if err != nil || u.Host == "" || !validHTTPSEndpoint {
		return ErrInvalidConfiguration
	}
	return nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
