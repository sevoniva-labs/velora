package provisioninghttp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
)

type Config struct {
	Enabled     bool
	URL, Secret string
	HTTPClient  *http.Client
}
type Dispatcher struct {
	enabled bool
	url     string
	secret  []byte
	client  *http.Client
}

func New(cfg Config) (*Dispatcher, error) {
	if !cfg.Enabled {
		return &Dispatcher{}, nil
	}
	u, err := url.Parse(strings.TrimSpace(cfg.URL))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("provisioning target must be an absolute HTTPS URL")
	}
	if len(strings.TrimSpace(cfg.Secret)) < 32 {
		return nil, errors.New("provisioning secret must contain at least 32 bytes")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Dispatcher{enabled: true, url: u.String(), secret: []byte(strings.TrimSpace(cfg.Secret)), client: client}, nil
}

func (d *Dispatcher) Enabled() bool { return d != nil && d.enabled }

func (d *Dispatcher) Publish(ctx context.Context, message messaging.Message) (string, error) {
	if !d.Enabled() {
		return "", errors.New("provisioning dispatcher is disabled")
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	mac := hmac.New(sha256.New, d.secret)
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(message.Body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url, bytes.NewReader(message.Body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Velora-Timestamp", timestamp)
	req.Header.Set("X-Velora-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-Request-ID", message.ID)
	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("provisioning target returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", errors.New("provisioning target returned invalid JSON")
	}
	if result.Status != "APPLIED" && result.Status != "STALE" && result.Status != "DUPLICATE" {
		return "", errors.New("provisioning target did not acknowledge event")
	}
	return message.ID, nil
}
