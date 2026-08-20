package identitysource

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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
	Name         string
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	AllowHTTP    bool
}

type OIDCProvider struct {
	name     string
	config   oauth2.Config
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
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
	if client == nil {
		client = http.DefaultClient
	}
	ctx = oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	scopes := append([]string{"openid", "profile", "email"}, cfg.Scopes...)
	return &OIDCProvider{
		name:     cfg.Name,
		config:   oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: cfg.RedirectURL, Scopes: uniqueStrings(scopes)},
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, nil
}

func (p *OIDCProvider) Name() string { return p.name }

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
