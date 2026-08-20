package identitysource

import (
	"bytes"
	"context"
	"crypto/tls"
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
	Subject          string
	LoginName        string
	DisplayName      string
	Email            string
	Groups           []string
	Provider         string
	AuthenticationAt time.Time
}

type OIDCConfig struct {
	Name                  string
	Issuer                string
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	PostLogoutRedirectURL string
	Scopes                []string
	AllowHTTP             bool
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
		Subject           string   `json:"sub"`
		Email             string   `json:"email"`
		Name              string   `json:"name"`
		PreferredUsername string   `json:"preferred_username"`
		Groups            []string `json:"groups"`
		Nonce             string   `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Nonce != nonce {
		return FederatedIdentity{}, ErrAuthenticationFailed
	}
	identity := FederatedIdentity{Subject: claims.Subject, LoginName: claims.PreferredUsername, DisplayName: claims.Name, Email: claims.Email, Groups: uniqueStrings(claims.Groups), Provider: p.name, AuthenticationAt: time.Now().UTC()}
	if identity.LoginName == "" {
		identity.LoginName = claims.Email
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.LoginName
	}
	return identity, nil
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
