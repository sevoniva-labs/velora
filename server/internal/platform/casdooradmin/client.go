// Package casdooradmin contains the deliberately narrow, feature-flagged
// Casdoor application automation client. It never manages users, passwords,
// roles or MFA and never persists or logs a Casdoor client secret.
package casdooradmin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrApprovalRequired = errors.New("maker-checker approval is required")
var ErrApplicationNotFound = errors.New("casdoor application not found")

type Config struct {
	BaseURL      string
	Token        string
	Owner        string
	Organization string
	Enabled      bool
	HTTPClient   *http.Client
}

type Application struct {
	Name                string
	Organization        string
	DisplayName         string
	ClientID            string
	RedirectURIs        []string
	GrantTypes          []string
	Scopes              []string
	Enabled             bool
	oneTimeClientSecret string
}

// TakeOneTimeClientSecret returns a newly-created Casdoor client secret once
// and clears the in-memory copy. It is intentionally absent from JSON models,
// database records, logs and audit payloads.
func (a *Application) TakeOneTimeClientSecret() string {
	if a == nil {
		return ""
	}
	secret := a.oneTimeClientSecret
	a.oneTimeClientSecret = ""
	return secret
}

type applicationWire struct {
	Name              string   `json:"name"`
	Organization      string   `json:"organization"`
	Owner             string   `json:"owner"`
	DisplayName       string   `json:"displayName"`
	ClientID          string   `json:"clientId"`
	RedirectURIs      []string `json:"redirectUris"`
	GrantTypes        []string `json:"grantTypes"`
	Scopes            []string `json:"scopes"`
	Enabled           bool     `json:"enableSigninSession"`
	ClientSecret      string   `json:"clientSecret"`
	ClientSecretSnake string   `json:"client_secret"`
}

func (w applicationWire) application(includeSecret bool) Application {
	organization := w.Organization
	if organization == "" {
		organization = w.Owner
	}
	secret := ""
	if includeSecret {
		secret = w.ClientSecret
		if secret == "" {
			secret = w.ClientSecretSnake
		}
	}
	return Application{Name: w.Name, Organization: organization, DisplayName: w.DisplayName, ClientID: w.ClientID, RedirectURIs: w.RedirectURIs, GrantTypes: w.GrantTypes, Scopes: w.Scopes, Enabled: w.Enabled, oneTimeClientSecret: secret}
}

type UpsertInput struct {
	Name         string
	Organization string
	DisplayName  string
	ClientID     string
	RedirectURIs []string
	GrantTypes   []string
	Scopes       []string
	ApprovalID   string
}

// ApplicationProvider is the narrow boundary Velora uses for external
// identity-client lifecycle management. Implementations must not expose APIs
// for users, passwords, roles, organizations or MFA.
type ApplicationProvider interface {
	Enabled() bool
	GetApplication(context.Context, string) (Application, bool, error)
	UpsertApplication(context.Context, UpsertInput) (Application, bool, error)
	DisableApplication(context.Context, string, string) error
}

type Client struct {
	baseURL      string
	token        string
	owner        string
	organization string
	enabled      bool
	httpClient   *http.Client
}

var _ ApplicationProvider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if !cfg.Enabled {
		return &Client{baseURL: base, enabled: false, httpClient: cfg.HTTPClient}, nil
	}
	if base == "" || strings.TrimSpace(cfg.Token) == "" || strings.TrimSpace(cfg.Owner) == "" || strings.TrimSpace(cfg.Organization) == "" {
		return nil, errors.New("casdoor automation requires base URL, owner, organization and a secret token")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !isLocalHTTP(u)) {
		return nil, errors.New("casdoor automation base URL must use HTTPS (local HTTP is allowed only on loopback)")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: base, token: strings.TrimSpace(cfg.Token), owner: strings.TrimSpace(cfg.Owner), organization: strings.TrimSpace(cfg.Organization), enabled: true, httpClient: client}, nil
}

func (c *Client) Enabled() bool { return c != nil && c.enabled }

func (c *Client) GetApplication(ctx context.Context, ref string) (Application, bool, error) {
	if !c.Enabled() {
		return Application{}, false, nil
	}
	var wire applicationWire
	id := c.applicationID(ref)
	status, err := c.do(ctx, http.MethodGet, "/api/get-application?id="+url.QueryEscape(id), nil, &wire)
	if status == http.StatusNotFound || errors.Is(err, ErrApplicationNotFound) {
		return Application{}, false, nil
	}
	if err != nil {
		return Application{}, false, err
	}
	// Casdoor v1.762 returns HTTP 200 with data:null for a missing application.
	// Decoding null leaves an empty wire value, which must not be mistaken for
	// an existing client or the following update becomes a silent no-op.
	if strings.TrimSpace(wire.Name) == "" {
		return Application{}, false, nil
	}
	return wire.application(false), true, nil
}

func (c *Client) UpsertApplication(ctx context.Context, input UpsertInput) (Application, bool, error) {
	if !c.Enabled() {
		return Application{}, false, errors.New("casdoor automation is disabled")
	}
	if strings.TrimSpace(input.ApprovalID) == "" {
		return Application{}, false, ErrApprovalRequired
	}
	if strings.TrimSpace(input.Name) == "" || len(input.RedirectURIs) == 0 {
		return Application{}, false, errors.New("application name and redirect URIs are required")
	}
	existing, found, err := c.GetApplication(ctx, input.Name)
	if err != nil {
		return Application{}, false, err
	}
	// Casdoor's authorization UI calls array helpers on these fields without
	// consistently guarding null. Always send empty arrays for optional
	// collections so a newly automated OIDC client cannot render a blank page.
	request := map[string]any{
		"owner": c.owner, "name": input.Name, "organization": c.organization,
		"displayName": input.DisplayName, "clientId": input.ClientID, "redirectUris": input.RedirectURIs,
		"grantTypes":          input.GrantTypes,
		"enableSigninSession": true, "enableAutoSignin": true,
		"providers": []any{}, "signupItems": []any{}, "signinItems": []any{},
		"tags": []string{}, "samlAttributes": []any{}, "tokenFields": []string{},
	}
	method, path := http.MethodPost, "/api/add-application"
	generatedSecret := ""
	if found {
		method = http.MethodPost
		path = "/api/update-application?id=" + url.QueryEscape(c.applicationID(input.Name)) + "&columns=" + url.QueryEscape("displayName,clientId,redirectUris,grantTypes,enableSigninSession,enableAutoSignin")
	} else {
		generatedSecret, err = randomSecret(48)
		if err != nil {
			return Application{}, false, err
		}
		request["clientSecret"] = generatedSecret
	}
	if _, err := c.do(ctx, method, path, request, nil); err != nil {
		return Application{}, false, err
	}
	// Only a newly created client may yield a one-time secret. Updates never
	// re-expose an existing secret, even if Casdoor includes it in the payload.
	application := Application{Name: input.Name, Organization: c.organization, DisplayName: input.DisplayName, ClientID: input.ClientID, RedirectURIs: append([]string(nil), input.RedirectURIs...), GrantTypes: append([]string(nil), input.GrantTypes...), Scopes: append([]string(nil), input.Scopes...), Enabled: true}
	if application.DisplayName == "" {
		application.DisplayName = existing.DisplayName
	}
	if !found {
		application.oneTimeClientSecret = generatedSecret
	}
	return application, !found, nil
}

func (c *Client) DisableApplication(ctx context.Context, ref, approvalID string) error {
	if !c.Enabled() {
		return errors.New("casdoor automation is disabled")
	}
	if strings.TrimSpace(approvalID) == "" {
		return ErrApprovalRequired
	}
	path := "/api/update-application?id=" + url.QueryEscape(c.applicationID(ref)) + "&columns=" + url.QueryEscape("redirectUris,enableSigninSession,enablePassword")
	_, err := c.do(ctx, http.MethodPost, path, map[string]any{"owner": c.owner, "name": strings.TrimPrefix(c.applicationID(ref), c.owner+"/"), "redirectUris": []string{}, "enableSigninSession": false, "enablePassword": false}, nil)
	return err
}

func (c *Client) applicationID(ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.Contains(ref, "/") {
		return ref
	}
	return c.owner + "/" + ref
}

func randomSecret(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// The token is never included in errors, logs or response objects.
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("casdoor API returned HTTP %d", resp.StatusCode)
	}
	if len(data) == 0 {
		return resp.StatusCode, nil
	}
	var envelope struct {
		Status  string          `json:"status"`
		Message string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return resp.StatusCode, errors.New("casdoor API returned invalid JSON")
	}
	if envelope.Status != "" && !strings.EqualFold(envelope.Status, "ok") && !strings.EqualFold(envelope.Status, "success") {
		message := strings.ToLower(strings.TrimSpace(envelope.Message))
		if strings.Contains(message, "does not exist") || strings.Contains(message, "not found") || strings.Contains(message, "不存在") {
			return resp.StatusCode, ErrApplicationNotFound
		}
		return resp.StatusCode, errors.New("casdoor API rejected the request")
	}
	if out == nil {
		return resp.StatusCode, nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return resp.StatusCode, nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return resp.StatusCode, errors.New("casdoor API response shape is unsupported")
	}
	return resp.StatusCode, nil
}

func isLocalHTTP(u *url.URL) bool {
	return u.Scheme == "http" && (strings.EqualFold(u.Hostname(), "casdoor") || strings.EqualFold(u.Hostname(), "localhost") || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}
