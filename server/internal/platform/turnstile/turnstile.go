// Package turnstile contains the server-side Cloudflare Turnstile verifier.
// Tokens are single-use and must never be logged or persisted.
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const verifyEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

type Config struct {
	Secret    string
	Action    string
	Hostnames []string
}

type Verifier struct {
	secret    string
	action    string
	hostnames map[string]struct{}
	client    *http.Client
	endpoint  string
}

type siteVerifyResponse struct {
	Success    bool     `json:"success"`
	Action     string   `json:"action"`
	Hostname   string   `json:"hostname"`
	ErrorCodes []string `json:"error-codes"`
}

func New(cfg Config) (*Verifier, error) {
	secret := strings.TrimSpace(cfg.Secret)
	action := strings.TrimSpace(cfg.Action)
	if secret == "" || action == "" || len(cfg.Hostnames) == 0 {
		return nil, errors.New("turnstile requires secret, action, and at least one hostname")
	}
	hostnames := make(map[string]struct{}, len(cfg.Hostnames))
	for _, raw := range cfg.Hostnames {
		host := strings.ToLower(strings.TrimSpace(raw))
		if host == "" {
			continue
		}
		hostnames[host] = struct{}{}
	}
	if len(hostnames) == 0 {
		return nil, errors.New("turnstile requires at least one non-empty hostname")
	}
	return &Verifier{
		secret: secret, action: action, hostnames: hostnames,
		client: &http.Client{Timeout: 10 * time.Second}, endpoint: verifyEndpoint,
	}, nil
}

// Verify checks success, the configured action, and the exact frontend
// hostname. It fails closed on network, protocol, or response validation
// errors and intentionally returns no provider detail to callers.
func (v *Verifier) Verify(ctx context.Context, token, remoteIP string) error {
	if v == nil || v.secret == "" || v.action == "" || len(v.hostnames) == 0 {
		return errors.New("turnstile verifier is not configured")
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 2048 {
		return errors.New("invalid turnstile token")
	}
	form := url.Values{}
	form.Set("secret", v.secret)
	form.Set("response", token)
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		form.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("turnstile verification request failed")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := v.client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("turnstile verification request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile verification returned status %d", resp.StatusCode)
	}
	var result siteVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.New("turnstile verification response was invalid")
	}
	if !result.Success || result.Action != v.action {
		return errors.New("turnstile verification failed")
	}
	if _, ok := v.hostnames[strings.ToLower(strings.TrimSpace(result.Hostname))]; !ok {
		return errors.New("turnstile hostname was not allowed")
	}
	return nil
}
