package velora

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDCConfig struct {
	Issuer, ClientID, ClientSecret, RedirectURL string
	Scopes                                      []string
}
type Authorization struct {
	URL, State, Nonce, PKCEVerifier string
	ExpiresAt                       time.Time
}
type Identity struct {
	Subject, Name, Email, RawIDToken string
	Expiry                           time.Time
}
type OIDCClient struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func NewOIDCClient(ctx context.Context, cfg OIDCConfig) (*OIDCClient, error) {
	issuer := strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
	redirect, err := url.Parse(strings.TrimSpace(cfg.RedirectURL))
	if err != nil || issuer == "" || cfg.ClientID == "" || cfg.ClientSecret == "" || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil || redirect.Fragment != "" {
		return nil, errors.New("invalid production OIDC configuration")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	scopes := append([]string(nil), cfg.Scopes...)
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	return &OIDCClient{oauth: oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, RedirectURL: redirect.String(), Endpoint: provider.Endpoint(), Scopes: scopes}, verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})}, nil
}

func (c *OIDCClient) NewAuthorization() (Authorization, error) {
	state, err := randomURLToken(32)
	if err != nil {
		return Authorization{}, err
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		return Authorization{}, err
	}
	verifier, err := randomURLToken(32)
	if err != nil {
		return Authorization{}, err
	}
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	u := c.oauth.AuthCodeURL(state, oauth2.SetAuthURLParam("nonce", nonce), oauth2.SetAuthURLParam("code_challenge", challenge), oauth2.SetAuthURLParam("code_challenge_method", "S256"))
	return Authorization{URL: u, State: state, Nonce: nonce, PKCEVerifier: verifier, ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}, nil
}

func (c *OIDCClient) Exchange(ctx context.Context, code, expectedNonce, verifier string) (Identity, error) {
	if strings.TrimSpace(code) == "" || expectedNonce == "" || verifier == "" {
		return Identity{}, errors.New("OIDC callback transaction is incomplete")
	}
	token, err := c.oauth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return Identity{}, err
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return Identity{}, errors.New("OIDC response has no ID token")
	}
	idToken, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, err
	}
	var claims struct {
		Subject string `json:"sub"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Nonce   string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Nonce != expectedNonce {
		return Identity{}, errors.New("OIDC claims or nonce are invalid")
	}
	return Identity{Subject: claims.Subject, Name: claims.Name, Email: claims.Email, RawIDToken: raw, Expiry: idToken.Expiry}, nil
}

func randomURLToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
