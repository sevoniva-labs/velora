package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/tlsx"
)

type Engine interface {
	Index(context.Context, string, string, any) error
	Search(context.Context, string, any) (json.RawMessage, error)
	Ping(context.Context) error
	Provider() string
}

func New(cfg config.Search) (Engine, error) {
	if cfg.Provider == "" || cfg.Provider == "disabled" {
		return noop{}, nil
	}
	if (cfg.Provider != "elasticsearch" && cfg.Provider != "opensearch") || len(cfg.URLs) == 0 {
		return nil, errors.New("invalid search configuration")
	}
	tlsCfg, err := tlsx.ClientConfig(tlsx.ClientOptions{
		Enabled: cfg.TLS, CAFile: cfg.TLSCAFile, CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, ServerName: cfg.TLSServerName,
	})
	if err != nil {
		return nil, err
	}
	if cfg.TLS {
		for _, u := range cfg.URLs {
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(u)), "https://") {
				return nil, fmt.Errorf("search tls=true requires https URL: %s", u)
			}
		}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if tlsCfg != nil {
		tr.TLSClientConfig = tlsCfg
	}
	return &elastic{provider: cfg.Provider, urls: cfg.URLs, user: cfg.Username, pass: cfg.Password, client: &http.Client{Timeout: 10 * time.Second, Transport: tr}}, nil
}

type noop struct{}

func (noop) Index(context.Context, string, string, any) error { return nil }
func (noop) Search(context.Context, string, any) (json.RawMessage, error) {
	return json.RawMessage(`{"hits":{"hits":[]}}`), nil
}
func (noop) Ping(context.Context) error { return nil }
func (noop) Provider() string           { return "disabled" }

type elastic struct {
	provider   string
	urls       []string
	user, pass string
	client     *http.Client
	rr         uint64
}

func (e *elastic) base() string {
	n := atomic.AddUint64(&e.rr, 1)
	index := int(n % uint64(len(e.urls))) // #nosec G115 -- modulo by the non-zero slice length guarantees the result fits int.
	return strings.TrimRight(e.urls[index], "/")
}
func (e *elastic) request(ctx context.Context, method, path string, body any) ([]byte, error) {
	var r io.Reader
	if body != nil {
		b, er := json.Marshal(body)
		if er != nil {
			return nil, er
		}
		r = bytes.NewReader(b)
	}
	req, er := http.NewRequestWithContext(ctx, method, e.base()+path, r)
	if er != nil {
		return nil, er
	}
	req.Header.Set("Content-Type", "application/json")
	if e.user != "" {
		req.SetBasicAuth(e.user, e.pass)
	}
	resp, er := e.client.Do(req)
	if er != nil {
		return nil, er
	}
	defer func() { _ = resp.Body.Close() }()
	b, er := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if er != nil {
		return nil, er
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s", e.provider, resp.Status, string(b))
	}
	return b, nil
}
func (e *elastic) Index(ctx context.Context, index, id string, doc any) error {
	_, er := e.request(ctx, http.MethodPut, "/"+index+"/_doc/"+id, doc)
	return er
}
func (e *elastic) Search(ctx context.Context, index string, q any) (json.RawMessage, error) {
	b, er := e.request(ctx, http.MethodPost, "/"+index+"/_search", q)
	return json.RawMessage(b), er
}
func (e *elastic) Ping(ctx context.Context) error {
	_, er := e.request(ctx, http.MethodGet, "/", nil)
	return er
}
func (e *elastic) Provider() string { return e.provider }
