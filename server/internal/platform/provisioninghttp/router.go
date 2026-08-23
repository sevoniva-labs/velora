package provisioninghttp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/messaging"
	"github.com/sevoniva-labs/velora/server/internal/platform/securefile"
)

const ProvisioningTopicPrefix = "velora.provisioning."

var ErrTargetNotConfigured = errors.New("provisioning target is not configured")

type Router struct {
	db     *database.DB
	client *http.Client
}

func NewRouter(db *database.DB, client *http.Client) (*Router, error) {
	if db == nil {
		return nil, errors.New("provisioning router database is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	return &Router{db: db, client: client}, nil
}

func (r *Router) Publish(ctx context.Context, message messaging.Message) (string, error) {
	applicationCode, err := applicationCodeFromTopic(message.Topic)
	if err != nil {
		return "", err
	}
	var endpoint, secretRef string
	err = r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT t.endpoint_url,t.secret_ref
		FROM portal_application_provisioning_targets t
		JOIN portal_applications a ON a.id=t.application_id AND a.organization_id=t.organization_id
		WHERE t.organization_id=? AND a.code=? AND t.delivery_status<>'DISABLED'`), message.OrganizationID, applicationCode).Scan(&endpoint, &secretRef)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTargetNotConfigured
	}
	if err != nil {
		return "", err
	}
	secret, err := readSecretReference(secretRef)
	if err != nil {
		r.recordFailure(ctx, message.OrganizationID, applicationCode, "SECRET_UNAVAILABLE")
		return "", fmt.Errorf("load provisioning signing key: %w", err)
	}
	dispatcher, err := New(Config{Enabled: true, URL: endpoint, Secret: secret, HTTPClient: r.client})
	if err != nil {
		r.recordFailure(ctx, message.OrganizationID, applicationCode, "TARGET_CONFIGURATION_INVALID")
		return "", err
	}
	id, err := dispatcher.Publish(ctx, message)
	if err != nil {
		r.recordFailure(ctx, message.OrganizationID, applicationCode, "DELIVERY_FAILED")
		return "", err
	}
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE portal_application_provisioning_targets t SET delivery_status='HEALTHY',last_success_at=?,last_error_code='',updated_at=? FROM portal_applications a WHERE t.application_id=a.id AND t.organization_id=a.organization_id AND t.organization_id=? AND a.code=?`), time.Now().UTC(), time.Now().UTC(), message.OrganizationID, applicationCode)
	return id, nil
}

func (r *Router) recordFailure(ctx context.Context, organizationID, applicationCode, code string) {
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE portal_application_provisioning_targets t SET delivery_status='DEGRADED',last_failure_at=?,last_error_code=?,updated_at=? FROM portal_applications a WHERE t.application_id=a.id AND t.organization_id=a.organization_id AND t.organization_id=? AND a.code=?`), time.Now().UTC(), code, time.Now().UTC(), organizationID, applicationCode)
}

func applicationCodeFromTopic(topic string) (string, error) {
	if !strings.HasPrefix(topic, ProvisioningTopicPrefix) {
		return "", errors.New("message is not a provisioning topic")
	}
	code := strings.TrimPrefix(topic, ProvisioningTopicPrefix)
	if code == "" || len(code) > 100 {
		return "", errors.New("provisioning topic has an invalid application code")
	}
	for _, r := range code {
		if r != '-' && r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return "", errors.New("provisioning topic has an invalid application code")
		}
	}
	return code, nil
}

func readSecretReference(reference string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || u.Scheme != "file" || u.Host != "" || !filepath.IsAbs(u.Path) || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("only an absolute file secret reference is supported")
	}
	raw, err := securefile.Read(u.Path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(raw))
	if len(secret) < 32 {
		return "", errors.New("provisioning secret must contain at least 32 bytes")
	}
	return secret, nil
}
