// Package casdooridentity provides the deliberately narrow Casdoor user
// lifecycle adapter. Application/client automation remains in casdooradmin.
package casdooridentity

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

	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
)

type Config struct {
	BaseURL, ClientID, ClientSecret, Organization, Application string
	Enabled                                                    bool
	HTTPClient                                                 *http.Client
}

type Client struct {
	baseURL, clientID, clientSecret, organization, application string
	enabled                                                    bool
	httpClient                                                 *http.Client
}

var errNotFound = errors.New("Casdoor identity not found")

var _ appidentity.ManagedIdentityProvider = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if !cfg.Enabled {
		return &Client{}, nil
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "casdoor" || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return nil, errors.New("Casdoor identity API must use HTTPS or an approved internal hostname")
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" || strings.TrimSpace(cfg.Organization) == "" {
		return nil, errors.New("Casdoor identity API requires client credentials and organization")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{baseURL: base, clientID: cfg.ClientID, clientSecret: cfg.ClientSecret, organization: cfg.Organization, application: cfg.Application, enabled: true, httpClient: httpClient}, nil
}

func (c *Client) Enabled() bool { return c != nil && c.enabled }

type userWire struct {
	Owner             string `json:"owner"`
	Name              string `json:"name"`
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	Email             string `json:"email"`
	Password          string `json:"password,omitempty"`
	Type              string `json:"type,omitempty"`
	SignupApplication string `json:"signupApplication,omitempty"`
	IsForbidden       bool   `json:"isForbidden"`
}

func (c *Client) CreateUser(ctx context.Context, in appidentity.ManagedUserInput) (string, error) {
	if !c.Enabled() {
		return "", errors.New("managed identity provider is disabled")
	}
	login := strings.TrimSpace(in.LoginName)
	if _, found, err := c.get(ctx, login); err != nil {
		return "", err
	} else if found {
		return "", fmt.Errorf("Casdoor user %q already exists", login)
	}
	u := userWire{Owner: c.organization, Name: login, DisplayName: strings.TrimSpace(in.DisplayName), Email: strings.TrimSpace(in.Email), Password: in.Password, Type: "normal-user", SignupApplication: c.application}
	if err := c.modify(ctx, "add-user", c.organization+"/"+login, nil, u); err != nil {
		return "", err
	}
	created, found, err := c.get(ctx, login)
	if err != nil {
		return "", err
	}
	if !found || created.ID == "" {
		return "", errors.New("Casdoor created user without a stable subject")
	}
	return created.ID, nil
}

func (c *Client) SetUserStatus(ctx context.Context, login string, active bool) error {
	u, found, err := c.get(ctx, login)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("Casdoor user not found")
	}
	u.Password = ""
	u.IsForbidden = !active
	if err := c.modify(ctx, "update-user", u.Owner+"/"+u.Name, []string{"isForbidden"}, u); err != nil {
		return err
	}
	if !active {
		return c.revokeUserAccess(ctx, login)
	}
	return nil
}

type sessionWire struct {
	Owner, Name, Application string
	SessionID                []string `json:"sessionId"`
}
type tokenWire struct{ Owner, Name, Organization, User string }

func (c *Client) revokeUserAccess(ctx context.Context, login string) error {
	var sessions []sessionWire
	if _, err := c.do(ctx, http.MethodGet, "/api/get-sessions?owner="+url.QueryEscape(c.organization), nil, &sessions); err != nil {
		return err
	}
	for _, session := range sessions {
		if session.Owner == c.organization && session.Name == login {
			if err := c.modifyAny(ctx, "delete-session", session.Owner+"/"+session.Name+"/"+session.Application, session); err != nil {
				return err
			}
		}
	}
	var tokens []tokenWire
	if _, err := c.do(ctx, http.MethodGet, "/api/get-tokens?owner=admin", nil, &tokens); err != nil {
		return err
	}
	for _, token := range tokens {
		if token.Organization == c.organization && token.User == login {
			if err := c.modifyAny(ctx, "delete-token", "admin/"+token.Name, token); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) modifyAny(ctx context.Context, action, id string, body any) error {
	_, err := c.do(ctx, http.MethodPost, "/api/"+action+"?id="+url.QueryEscape(id), body, nil)
	return err
}

func (c *Client) SetUserPassword(ctx context.Context, login, password string) error {
	u, found, err := c.get(ctx, login)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("Casdoor user not found")
	}
	u.Password = password
	if err := c.modify(ctx, "update-user", u.Owner+"/"+u.Name, []string{"password"}, u); err != nil {
		return err
	}
	return c.revokeUserAccess(ctx, login)
}

func (c *Client) get(ctx context.Context, login string) (userWire, bool, error) {
	var out userWire
	status, err := c.do(ctx, http.MethodGet, "/api/get-user?id="+url.QueryEscape(c.organization+"/"+strings.TrimSpace(login)), nil, &out)
	if status == http.StatusNotFound {
		return out, false, nil
	}
	if errors.Is(err, errNotFound) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	return out, out.Name != "", nil
}

func (c *Client) modify(ctx context.Context, action, id string, columns []string, body userWire) error {
	path := "/api/" + action + "?id=" + url.QueryEscape(id)
	if len(columns) > 0 {
		path += "&columns=" + url.QueryEscape(strings.Join(columns, ","))
	}
	_, err := c.do(ctx, http.MethodPost, path, body, nil)
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
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
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
		return resp.StatusCode, fmt.Errorf("Casdoor identity API returned HTTP %d", resp.StatusCode)
	}
	var envelope struct {
		Status, Msg string
		Data        json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return resp.StatusCode, errors.New("Casdoor identity API returned invalid JSON")
	}
	if !strings.EqualFold(envelope.Status, "ok") {
		message := strings.ToLower(envelope.Msg)
		if strings.Contains(message, "not exist") || strings.Contains(message, "not found") {
			return resp.StatusCode, errNotFound
		}
		return resp.StatusCode, errors.New("Casdoor identity API rejected the request")
	}
	if out != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return resp.StatusCode, errors.New("Casdoor identity API response shape is unsupported")
		}
	}
	return resp.StatusCode, nil
}
