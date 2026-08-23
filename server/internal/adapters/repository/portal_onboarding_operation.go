package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
)

type ProviderReconciliationCandidate struct {
	Application portaldomain.Application
	Binding     portaldomain.IdentityBinding
}

func (r *PortalRepo) ListProviderReconciliationCandidates(ctx context.Context, limit int) ([]ProviderReconciliationCandidate, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT a.id,a.organization_id,a.code,a.status,a.lifecycle_status,a.config_version,b.id,b.provider_key,b.protocol,b.provider_application_ref,b.public_client_id,b.issuer,b.redirect_uris_json,b.scopes_json,b.configuration_status,b.verification_status,b.config_version FROM portal_applications a JOIN portal_application_identity_bindings b ON b.application_id=a.id AND b.organization_id=a.organization_id WHERE b.provider_key='casdoor' AND b.protocol='OIDC' ORDER BY a.updated_at LIMIT ?`), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]ProviderReconciliationCandidate, 0)
	for rows.Next() {
		var item ProviderReconciliationCandidate
		var redirectsJSON, scopesJSON string
		if err := rows.Scan(&item.Application.ID, &item.Application.OrganizationID, &item.Application.Code, &item.Application.Status, &item.Application.LifecycleStatus, &item.Application.ConfigVersion, &item.Binding.ID, &item.Binding.ProviderKey, &item.Binding.Protocol, &item.Binding.ProviderApplicationRef, &item.Binding.PublicClientID, &item.Binding.Issuer, &redirectsJSON, &scopesJSON, &item.Binding.ConfigurationStatus, &item.Binding.VerificationStatus, &item.Binding.ConfigVersion); err != nil {
			return nil, err
		}
		item.Binding.OrganizationID, item.Binding.ApplicationID = item.Application.OrganizationID, item.Application.ID
		if json.Unmarshal([]byte(redirectsJSON), &item.Binding.RedirectURIs) != nil || json.Unmarshal([]byte(scopesJSON), &item.Binding.Scopes) != nil {
			continue
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PortalRepo) BeginOnboardingOperation(ctx context.Context, organizationID, applicationID, operationType string, desiredVersion int64, idempotencyKey string) (portaldomain.OnboardingOperation, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	query := `INSERT INTO portal_application_onboarding_operations(id,organization_id,application_id,operation_type,desired_version,status,idempotency_key,attempt_count,result_summary_json,created_at,updated_at) VALUES(?,?,?,?,?,'RUNNING',?,1,'{}',?,?) ON CONFLICT (organization_id,idempotency_key) DO UPDATE SET status='RUNNING',attempt_count=portal_application_onboarding_operations.attempt_count+1,error_code='',next_retry_at=NULL,updated_at=EXCLUDED.updated_at`
	if r.db.Provider != "postgres" {
		query = `INSERT INTO portal_application_onboarding_operations(id,organization_id,application_id,operation_type,desired_version,status,idempotency_key,attempt_count,result_summary_json,created_at,updated_at) VALUES(?,?,?,?,?,'RUNNING',?,1,'{}',?,?) ON DUPLICATE KEY UPDATE status='RUNNING',attempt_count=attempt_count+1,error_code='',next_retry_at=NULL,updated_at=VALUES(updated_at)`
	}
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(query), id, organizationID, applicationID, operationType, desiredVersion, idempotencyKey, now, now); err != nil {
		return portaldomain.OnboardingOperation{}, err
	}
	return r.GetOnboardingOperationByKey(ctx, organizationID, idempotencyKey)
}

func (r *PortalRepo) GetOnboardingOperationByKey(ctx context.Context, organizationID, idempotencyKey string) (portaldomain.OnboardingOperation, error) {
	return r.scanOnboardingOperation(r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,application_id,operation_type,desired_version,status,idempotency_key,attempt_count,provider_request_id,result_summary_json,error_code,next_retry_at,created_at,updated_at,completed_at FROM portal_application_onboarding_operations WHERE organization_id=? AND idempotency_key=?`), organizationID, idempotencyKey))
}

func (r *PortalRepo) LatestOnboardingOperation(ctx context.Context, organizationID, applicationID string) (portaldomain.OnboardingOperation, error) {
	return r.scanOnboardingOperation(r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,application_id,operation_type,desired_version,status,idempotency_key,attempt_count,provider_request_id,result_summary_json,error_code,next_retry_at,created_at,updated_at,completed_at FROM portal_application_onboarding_operations WHERE organization_id=? AND application_id=? ORDER BY updated_at DESC,id DESC LIMIT 1`), organizationID, applicationID))
}

func (r *PortalRepo) CompleteOnboardingOperation(ctx context.Context, id, status, errorCode, summary string, retryAt *time.Time) error {
	var completed any
	if status == portaldomain.OperationSucceeded || status == portaldomain.OperationActionRequired {
		completed = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE portal_application_onboarding_operations SET status=?,error_code=?,result_summary_json=?,next_retry_at=?,completed_at=?,updated_at=? WHERE id=?`), status, errorCode, summary, retryAt, completed, time.Now().UTC(), id)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return nil
}

type rowScanner interface{ Scan(...any) error }

func (r *PortalRepo) scanOnboardingOperation(row rowScanner) (portaldomain.OnboardingOperation, error) {
	var item portaldomain.OnboardingOperation
	var retryAt, completedAt sql.NullTime
	err := row.Scan(&item.ID, &item.OrganizationID, &item.ApplicationID, &item.OperationType, &item.DesiredVersion, &item.Status, &item.IdempotencyKey, &item.AttemptCount, &item.ProviderRequestID, &item.ResultSummaryJSON, &item.ErrorCode, &retryAt, &item.CreatedAt, &item.UpdatedAt, &completedAt)
	if err != nil {
		return portaldomain.OnboardingOperation{}, err
	}
	if retryAt.Valid {
		item.NextRetryAt = &retryAt.Time
	}
	if completedAt.Valid {
		item.CompletedAt = &completedAt.Time
	}
	return item, nil
}

func IsOperationNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
