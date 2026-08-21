// Package casdooradmin contains the deliberately narrow, feature-flagged
// Casdoor application automation client. It never manages users, passwords,
// roles or MFA and never persists or logs a Casdoor client secret.
package casdooradmin

import (
	"bytes"
	"context"
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

type Config struct {
	BaseURL    string
	Token      string
	Enabled    bool
	HTTPClient *http.Client
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

type Client struct {
	baseURL    string
	token      string
	enabled    bool
	httpClient *http.Client
}

func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if !cfg.Enabled {
		return &Client{baseURL: base, enabled: false, httpClient: cfg.HTTPClient}, nil
	}
	if base == "" || strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("Casdoor automation requires base URL and a secret token")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !isLocalHTTP(u)) {
		return nil, errors.New("Casdoor automation base URL must use HTTPS (local HTTP is allowed only on loopback)")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: base, token: strings.TrimSpace(cfg.Token), enabled: true, httpClient: client}, nil
}

func (c *Client) Enabled() bool { return c != nil && c.enabled }

func (c *Client) GetApplication(ctx context.Context, ref string) (Application, bool, error) {
	if !c.Enabled() {
		return Application{}, false, nil
	}
	var wire applicationWire
	status, err := c.do(ctx, http.MethodGet, "/api/get-application?id="+url.QueryEscape(strings.TrimSpace(ref)), nil, &wire)
	if status == http.StatusNotFound {
		return Application{}, false, nil
	}
	if err != nil {
		return Application{}, false, err
	}
	return wire.application(false), true, nil
}

func (c *Client) UpsertApplication(ctx context.Context, input UpsertInput) (Application, bool, error) {
	if !c.Enabled() {
		return Application{}, false, errors.New("Casdoor automation is disabled")
	}
	if strings.TrimSpace(input.ApprovalID) == "" {
		return Application{}, false, ErrApprovalRequired
	}
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Organization) == "" || len(input.RedirectURIs) == 0 {
		return Application{}, false, errors.New("application name, organization and redirect URIs are required")
	}
	existing, found, err := c.GetApplication(ctx, input.Name)
	if err != nil {
		return Application{}, false, err
	}
	request := map[string]any{"owner": input.Organization, "name": input.Name, "organization": input.Organization, "displayName": input.DisplayName, "clientId": input.ClientID, "redirectUris": input.RedirectURIs, "grantTypes": input.GrantTypes, "scopes": input.Scopes, "enableSigninSession": true}
	method, path := http.MethodPost, "/api/add-application"
	if found {
		method, path = http.MethodPost, "/api/update-application"
		request["id"] = existing.Name
	}
	var wire applicationWire
	if _, err := c.do(ctx, method, path, request, &wire); err != nil {
		return Application{}, false, err
	}
	// Only a newly created client may yield a one-time secret. Updates never
	// re-expose an existing secret, even if Casdoor includes it in the payload.
	application := wire.application(!found)
	if application.Name == "" {
		application = Application{Name: input.Name, Organization: input.Organization, ClientID: input.ClientID, RedirectURIs: input.RedirectURIs, GrantTypes: input.GrantTypes, Scopes: input.Scopes, Enabled: true}
	}
	return application, !found, nil
}

func (c *Client) DisableApplication(ctx context.Context, ref, approvalID string) error {
	if !c.Enabled() {
		return errors.New("Casdoor automation is disabled")
	}
	if strings.TrimSpace(approvalID) == "" {
		return ErrApprovalRequired
	}
	_, err := c.do(ctx, http.MethodPost, "/api/update-application", map[string]any{"id": ref, "enableSigninSession": false, "enablePassword": false}, &struct{}{})
	return err
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
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("Casdoor API returned HTTP %d", resp.StatusCode)
	}
	if len(data) == 0 || out == nil {
		return resp.StatusCode, nil
	}
	var envelope struct {
		Status  string          `json:"status"`
		Message string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return resp.StatusCode, errors.New("Casdoor API returned invalid JSON")
	}
	if envelope.Status != "" && !strings.EqualFold(envelope.Status, "ok") && !strings.EqualFold(envelope.Status, "success") {
		return resp.StatusCode, errors.New("Casdoor API rejected the request")
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return resp.StatusCode, nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return resp.StatusCode, errors.New("Casdoor API response shape is unsupported")
	}
	return resp.StatusCode, nil
}

func isLocalHTTP(u *url.URL) bool {
	return u.Scheme == "http" && (strings.EqualFold(u.Hostname(), "localhost") || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}
