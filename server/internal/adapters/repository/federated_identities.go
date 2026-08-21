package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func (r *IdentityRepo) ListFederatedIdentityLinks(ctx context.Context, organizationID string) ([]identity.FederatedIdentityLink, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT e.id,e.organization_id,e.provider,e.subject,e.user_id,u.login_name,e.created_by,e.approval_id,e.created_at,e.last_authenticated_at FROM external_identities e JOIN users u ON u.id=e.user_id WHERE e.organization_id=? ORDER BY e.created_at DESC`), organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	links := make([]identity.FederatedIdentityLink, 0)
	for rows.Next() {
		var link identity.FederatedIdentityLink
		if err := rows.Scan(&link.ID, &link.OrganizationID, &link.Provider, &link.Subject, &link.UserID, &link.LoginName, &link.CreatedBy, &link.ApprovalID, &link.CreatedAt, &link.LastAuthenticatedAt); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (r *IdentityRepo) FederatedIdentityByProviderSubject(ctx context.Context, organizationID, provider, subject string) (identity.FederatedIdentityLink, error) {
	var link identity.FederatedIdentityLink
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT e.id,e.organization_id,e.provider,e.subject,e.user_id,u.login_name,e.created_by,e.approval_id,e.created_at,e.last_authenticated_at FROM external_identities e JOIN users u ON u.id=e.user_id WHERE e.organization_id=? AND e.provider=? AND e.subject=?`), organizationID, provider, subject).Scan(&link.ID, &link.OrganizationID, &link.Provider, &link.Subject, &link.UserID, &link.LoginName, &link.CreatedBy, &link.ApprovalID, &link.CreatedAt, &link.LastAuthenticatedAt)
	return link, err
}

func (r *IdentityRepo) CreateFederatedIdentityLink(ctx context.Context, link identity.FederatedIdentityLink) (identity.FederatedIdentityLink, error) {
	link.ID = uuid.NewString()
	link.Provider = strings.ToLower(strings.TrimSpace(link.Provider))
	link.Subject = strings.TrimSpace(link.Subject)
	link.CreatedAt = link.CreatedAt.UTC()
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO external_identities(id,organization_id,provider,subject,user_id,created_by,approval_id,created_at) VALUES(?,?,?,?,?,?,?,?)`), link.ID, link.OrganizationID, link.Provider, link.Subject, link.UserID, link.CreatedBy, link.ApprovalID, link.CreatedAt)
	if err != nil {
		return identity.FederatedIdentityLink{}, err
	}
	return r.FederatedIdentityByProviderSubject(ctx, link.OrganizationID, link.Provider, link.Subject)
}

func (r *IdentityRepo) DeleteFederatedIdentityLink(ctx context.Context, organizationID, linkID string) error {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM external_identities WHERE organization_id=? AND id=?`), organizationID, linkID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *IdentityRepo) TouchFederatedIdentityAuthentication(ctx context.Context, organizationID, linkID string, authenticatedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE external_identities SET last_authenticated_at=? WHERE organization_id=? AND id=?`), authenticatedAt.UTC(), organizationID, linkID)
	return err
}
