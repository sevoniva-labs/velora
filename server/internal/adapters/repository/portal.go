package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type PortalRepo struct{ db *database.DB }

func NewPortalRepo(db *database.DB) *PortalRepo { return &PortalRepo{db: db} }

type ApplicationFilter struct {
	Keyword       string
	CategoryID    string
	TagID         string
	FavoritesOnly bool
	Limit         int
}

type ApplicationInput struct {
	Code        string
	Name        string
	Description string
	Icon        string
	CategoryID  string
	HomeURL     string
	LaunchURL   string
	LaunchType  string
	Status      string
	SortOrder   int
	Featured    bool
	TagIDs      []string
}

type CategoryInput struct {
	Key         string
	Name        string
	Description string
	Status      string
	SortOrder   int
}

type TagInput struct {
	Key       string
	Name      string
	SortOrder int
}

func (r *PortalRepo) ListApplications(ctx context.Context, orgID, userID string, f ApplicationFilter, includeDisabled bool) ([]portaldomain.Application, error) {
	query := `SELECT a.id,a.organization_id,a.code,a.name,a.description,a.icon,COALESCE(a.category_id,''),COALESCE(c.name,''),a.home_url,a.launch_url,a.launch_type,a.status,a.sort_order,a.featured,a.created_by,a.updated_by,a.created_at,a.updated_at,
		CASE WHEN EXISTS (SELECT 1 FROM portal_favorites f WHERE f.organization_id=a.organization_id AND f.user_id=? AND f.application_id=a.id) THEN 1 ELSE 0 END,
		COALESCE((SELECT v.visit_count FROM portal_visits v WHERE v.organization_id=a.organization_id AND v.user_id=? AND v.application_id=a.id),0),
		a.lifecycle_status,a.published_at,a.published_by,a.config_version
		FROM portal_applications a LEFT JOIN portal_categories c ON c.id=a.category_id AND c.organization_id=a.organization_id WHERE a.organization_id=?`
	args := []any{userID, userID, orgID}
	if !includeDisabled {
		query += ` AND a.status=?`
		args = append(args, portaldomain.StatusEnabled)
	}
	if keyword := strings.TrimSpace(f.Keyword); keyword != "" {
		query += ` AND (LOWER(a.code) LIKE LOWER(?) OR LOWER(a.name) LIKE LOWER(?) OR LOWER(a.description) LIKE LOWER(?))`
		needle := "%" + keyword + "%"
		args = append(args, needle, needle, needle)
	}
	if id := strings.TrimSpace(f.CategoryID); id != "" {
		query += ` AND a.category_id=?`
		args = append(args, id)
	}
	if id := strings.TrimSpace(f.TagID); id != "" {
		query += ` AND EXISTS (SELECT 1 FROM portal_application_tags pt WHERE pt.application_id=a.id AND pt.tag_id=?)`
		args = append(args, id)
	}
	if f.FavoritesOnly {
		query += ` AND EXISTS (SELECT 1 FROM portal_favorites f WHERE f.organization_id=a.organization_id AND f.user_id=? AND f.application_id=a.id)`
		args = append(args, userID)
	}
	query += ` ORDER BY a.featured DESC,a.sort_order,a.name,a.id`
	if f.Limit <= 0 {
		f.Limit = 100
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	query += ` LIMIT ?`
	args = append(args, f.Limit)
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]portaldomain.Application, 0)
	for rows.Next() {
		var item portaldomain.Application
		var favorite int
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Code, &item.Name, &item.Description, &item.Icon, &item.CategoryID, &item.CategoryName, &item.HomeURL, &item.LaunchURL, &item.LaunchType, &item.Status, &item.SortOrder, &item.Featured, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &favorite, &item.VisitCount, &item.LifecycleStatus, &item.PublishedAt, &item.PublishedBy, &item.ConfigVersion); err != nil {
			return nil, err
		}
		item.Favorite = favorite == 1
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// A PostgreSQL transaction is pinned to one connection and cannot execute
	// relation queries while this result set is still open. Buffer the base rows
	// first, then load tags/policies after closing the cursor. This also keeps
	// the same behavior for MySQL and prevents transaction-only driver errors.
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range items {
		if err := r.loadRelations(ctx, &items[i]); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (r *PortalRepo) GetApplication(ctx context.Context, orgID, userID, id string, includeDisabled bool) (portaldomain.Application, error) {
	items, err := r.ListApplications(ctx, orgID, userID, ApplicationFilter{Limit: 500}, includeDisabled)
	if err != nil {
		return portaldomain.Application{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return portaldomain.Application{}, sql.ErrNoRows
}

func (r *PortalRepo) loadRelations(ctx context.Context, item *portaldomain.Application) error {
	tagRows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT t.id,t.organization_id,t.tag_key,t.name,t.sort_order,t.created_at,t.updated_at FROM portal_tags t JOIN portal_application_tags pt ON pt.tag_id=t.id WHERE pt.application_id=? ORDER BY t.sort_order,t.tag_key`), item.ID)
	if err != nil {
		return err
	}
	for tagRows.Next() {
		var tag portaldomain.Tag
		if err := tagRows.Scan(&tag.ID, &tag.OrganizationID, &tag.Key, &tag.Name, &tag.SortOrder, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			_ = tagRows.Close()
			return err
		}
		item.Tags = append(item.Tags, tag)
	}
	if err := tagRows.Err(); err != nil {
		_ = tagRows.Close()
		return err
	}
	_ = tagRows.Close()
	policyRows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,application_id,policy_type,value,created_at,updated_at FROM portal_access_policies WHERE application_id=? ORDER BY policy_type,value,id`), item.ID)
	if err != nil {
		return err
	}
	defer func() { _ = policyRows.Close() }()
	for policyRows.Next() {
		var policy portaldomain.AccessPolicy
		if err := policyRows.Scan(&policy.ID, &policy.ApplicationID, &policy.Type, &policy.Value, &policy.CreatedAt, &policy.UpdatedAt); err != nil {
			return err
		}
		item.Policies = append(item.Policies, policy)
	}
	return policyRows.Err()
}

func (r *PortalRepo) ListGroupKeys(ctx context.Context, orgID, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT ug.group_key FROM user_groups ug JOIN user_group_members ugm ON ugm.group_id=ug.id WHERE ug.organization_id=? AND ugm.user_id=? AND ug.status='ACTIVE' ORDER BY ug.group_key`), orgID, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (r *PortalRepo) CreateApplication(ctx context.Context, orgID, actorID string, input ApplicationInput) (portaldomain.Application, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		if _, err := r.db.ExecContext(txCtx, r.db.Rebind(`INSERT INTO portal_applications(id,organization_id,code,name,description,icon,category_id,home_url,launch_url,launch_type,status,sort_order,featured,created_by,updated_by,created_at,updated_at,lifecycle_status,config_version) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), id, orgID, input.Code, input.Name, input.Description, input.Icon, nullIfEmpty(input.CategoryID), input.HomeURL, input.LaunchURL, input.LaunchType, input.Status, input.SortOrder, input.Featured, actorID, actorID, now, now, portaldomain.LifecyclePublished, 1); err != nil {
			return err
		}
		return r.replaceTags(txCtx, orgID, id, input.TagIDs)
	})
	if err != nil {
		return portaldomain.Application{}, err
	}
	return r.GetApplication(ctx, orgID, actorID, id, true)
}

func (r *PortalRepo) UpdateApplication(ctx context.Context, orgID, actorID, id string, input ApplicationInput) (portaldomain.Application, error) {
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		res, err := r.db.ExecContext(txCtx, r.db.Rebind(`UPDATE portal_applications SET name=?,description=?,icon=?,category_id=?,home_url=?,launch_url=?,launch_type=?,status=?,sort_order=?,featured=?,updated_by=?,updated_at=?,config_version=config_version+1 WHERE organization_id=? AND id=?`), input.Name, input.Description, input.Icon, nullIfEmpty(input.CategoryID), input.HomeURL, input.LaunchURL, input.LaunchType, input.Status, input.SortOrder, input.Featured, actorID, time.Now().UTC(), orgID, id)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			if err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		return r.replaceTags(txCtx, orgID, id, input.TagIDs)
	})
	if err != nil {
		return portaldomain.Application{}, err
	}
	return r.GetApplication(ctx, orgID, actorID, id, true)
}

func (r *PortalRepo) DeleteApplication(ctx context.Context, orgID, id string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM portal_applications WHERE organization_id=? AND id=?`), orgID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PortalRepo) replaceTags(ctx context.Context, orgID, appID string, tagIDs []string) error {
	if _, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM portal_application_tags WHERE application_id=?`), appID); err != nil {
		return err
	}
	for _, tagID := range uniqueNonEmpty(tagIDs) {
		var count int
		if err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM portal_tags WHERE organization_id=? AND id=?`), orgID, tagID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("portal tag %s does not belong to organization", tagID)
		}
		if _, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO portal_application_tags(application_id,tag_id) VALUES(?,?)`), appID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func (r *PortalRepo) ListCategories(ctx context.Context, orgID string) ([]portaldomain.Category, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,category_key,name,description,status,sort_order,created_at,updated_at FROM portal_categories WHERE organization_id=? ORDER BY sort_order,category_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []portaldomain.Category
	for rows.Next() {
		var item portaldomain.Category
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Key, &item.Name, &item.Description, &item.Status, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PortalRepo) CreateCategory(ctx context.Context, orgID string, input CategoryInput) (portaldomain.Category, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO portal_categories(id,organization_id,category_key,name,description,status,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`), id, orgID, input.Key, input.Name, input.Description, input.Status, input.SortOrder, now, now)
	if err != nil {
		return portaldomain.Category{}, err
	}
	return r.categoryByID(ctx, orgID, id)
}

func (r *PortalRepo) UpdateCategory(ctx context.Context, orgID, id string, input CategoryInput) (portaldomain.Category, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE portal_categories SET name=?,description=?,status=?,sort_order=?,updated_at=? WHERE organization_id=? AND id=?`), input.Name, input.Description, input.Status, input.SortOrder, time.Now().UTC(), orgID, id)
	if err != nil {
		return portaldomain.Category{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return portaldomain.Category{}, err
	} else if n != 1 {
		return portaldomain.Category{}, sql.ErrNoRows
	}
	return r.categoryByID(ctx, orgID, id)
}

func (r *PortalRepo) DeleteCategory(ctx context.Context, orgID, id string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM portal_categories WHERE organization_id=? AND id=?`), orgID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PortalRepo) categoryByID(ctx context.Context, orgID, id string) (portaldomain.Category, error) {
	var item portaldomain.Category
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,category_key,name,description,status,sort_order,created_at,updated_at FROM portal_categories WHERE organization_id=? AND id=?`), orgID, id).Scan(&item.ID, &item.OrganizationID, &item.Key, &item.Name, &item.Description, &item.Status, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (r *PortalRepo) ListTags(ctx context.Context, orgID string) ([]portaldomain.Tag, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,tag_key,name,sort_order,created_at,updated_at FROM portal_tags WHERE organization_id=? ORDER BY sort_order,tag_key`), orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []portaldomain.Tag
	for rows.Next() {
		var item portaldomain.Tag
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Key, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *PortalRepo) CreateTag(ctx context.Context, orgID string, input TagInput) (portaldomain.Tag, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO portal_tags(id,organization_id,tag_key,name,sort_order,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`), id, orgID, input.Key, input.Name, input.SortOrder, now, now)
	if err != nil {
		return portaldomain.Tag{}, err
	}
	return r.tagByID(ctx, orgID, id)
}

func (r *PortalRepo) UpdateTag(ctx context.Context, orgID, id string, input TagInput) (portaldomain.Tag, error) {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`UPDATE portal_tags SET name=?,sort_order=?,updated_at=? WHERE organization_id=? AND id=?`), input.Name, input.SortOrder, time.Now().UTC(), orgID, id)
	if err != nil {
		return portaldomain.Tag{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return portaldomain.Tag{}, err
	} else if n != 1 {
		return portaldomain.Tag{}, sql.ErrNoRows
	}
	return r.tagByID(ctx, orgID, id)
}

func (r *PortalRepo) DeleteTag(ctx context.Context, orgID, id string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM portal_tags WHERE organization_id=? AND id=?`), orgID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PortalRepo) tagByID(ctx context.Context, orgID, id string) (portaldomain.Tag, error) {
	var item portaldomain.Tag
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,tag_key,name,sort_order,created_at,updated_at FROM portal_tags WHERE organization_id=? AND id=?`), orgID, id).Scan(&item.ID, &item.OrganizationID, &item.Key, &item.Name, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (r *PortalRepo) ReplacePolicies(ctx context.Context, orgID, applicationID string, policies []portaldomain.AccessPolicy) ([]portaldomain.AccessPolicy, error) {
	var out []portaldomain.AccessPolicy
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		var count int
		if err := r.db.QueryRowContext(txCtx, r.db.Rebind(`SELECT COUNT(*) FROM portal_applications WHERE organization_id=? AND id=?`), orgID, applicationID).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return sql.ErrNoRows
		}
		if _, err := r.db.ExecContext(txCtx, r.db.Rebind(`DELETE FROM portal_access_policies WHERE application_id=?`), applicationID); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, policy := range policies {
			policy.ID = uuid.NewString()
			policy.ApplicationID = applicationID
			policy.Type = strings.ToUpper(strings.TrimSpace(policy.Type))
			if !validPolicyType(policy.Type) || strings.TrimSpace(policy.Value) == "" && policy.Type != portaldomain.PolicyEveryone {
				return fmt.Errorf("invalid portal policy")
			}
			if _, err := r.db.ExecContext(txCtx, r.db.Rebind(`INSERT INTO portal_access_policies(id,application_id,policy_type,value,created_at,updated_at) VALUES(?,?,?,?,?,?)`), policy.ID, applicationID, policy.Type, strings.TrimSpace(policy.Value), now, now); err != nil {
				return err
			}
			policy.CreatedAt, policy.UpdatedAt = now, now
			out = append(out, policy)
		}
		return nil
	})
	return out, err
}

func (r *PortalRepo) AddFavorite(ctx context.Context, orgID, userID, applicationID string) error {
	query := `INSERT INTO portal_favorites(organization_id,user_id,application_id,created_at) VALUES(?,?,?,?)`
	if r.db.Provider == "postgres" {
		query += ` ON CONFLICT (organization_id,user_id,application_id) DO NOTHING`
	} else {
		query = `INSERT IGNORE INTO portal_favorites(organization_id,user_id,application_id,created_at) VALUES(?,?,?,?)`
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), orgID, userID, applicationID, time.Now().UTC())
	return err
}

func (r *PortalRepo) RemoveFavorite(ctx context.Context, orgID, userID, applicationID string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM portal_favorites WHERE organization_id=? AND user_id=? AND application_id=?`), orgID, userID, applicationID)
	return err
}

func (r *PortalRepo) RecordVisit(ctx context.Context, orgID, userID, applicationID string) error {
	now := time.Now().UTC()
	query := `INSERT INTO portal_visits(organization_id,user_id,application_id,visit_count,last_visited_at) VALUES(?,?,?,?,?) ON CONFLICT (organization_id,user_id,application_id) DO UPDATE SET visit_count=portal_visits.visit_count+1,last_visited_at=EXCLUDED.last_visited_at`
	if r.db.Provider != "postgres" {
		query = `INSERT INTO portal_visits(organization_id,user_id,application_id,visit_count,last_visited_at) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE visit_count=visit_count+1,last_visited_at=VALUES(last_visited_at)`
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(query), orgID, userID, applicationID, 1, now)
	return err
}

func (r *PortalRepo) ListFavorites(ctx context.Context, orgID, userID string, limit int) ([]portaldomain.Application, error) {
	return r.listByVisitOrFavorite(ctx, orgID, userID, limit, true)
}

func (r *PortalRepo) ListRecent(ctx context.Context, orgID, userID string, limit int) ([]portaldomain.Application, error) {
	return r.listByVisitOrFavorite(ctx, orgID, userID, limit, false)
}

func (r *PortalRepo) listByVisitOrFavorite(ctx context.Context, orgID, userID string, limit int, favorite bool) ([]portaldomain.Application, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	order := `v.last_visited_at DESC`
	condition := `EXISTS (SELECT 1 FROM portal_favorites f WHERE f.organization_id=a.organization_id AND f.user_id=? AND f.application_id=a.id)`
	if !favorite {
		condition = `v.user_id=?`
		order = `v.last_visited_at DESC`
	}
	query := `SELECT a.id,a.organization_id,a.code,a.name,a.description,a.icon,COALESCE(a.category_id,''),COALESCE(c.name,''),a.home_url,a.launch_url,a.launch_type,a.status,a.sort_order,a.featured,a.created_by,a.updated_by,a.created_at,a.updated_at,1,COALESCE(v.visit_count,0),a.lifecycle_status,a.published_at,a.published_by,a.config_version FROM portal_applications a LEFT JOIN portal_categories c ON c.id=a.category_id AND c.organization_id=a.organization_id LEFT JOIN portal_visits v ON v.organization_id=a.organization_id AND v.user_id=? AND v.application_id=a.id WHERE a.organization_id=? AND a.status=? AND ` + condition + ` ORDER BY ` + order + ` LIMIT ?`
	args := []any{userID, orgID, portaldomain.StatusEnabled, userID, limit}
	if favorite {
		args = []any{userID, orgID, portaldomain.StatusEnabled, userID, limit}
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []portaldomain.Application
	for rows.Next() {
		var item portaldomain.Application
		var fav int
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Code, &item.Name, &item.Description, &item.Icon, &item.CategoryID, &item.CategoryName, &item.HomeURL, &item.LaunchURL, &item.LaunchType, &item.Status, &item.SortOrder, &item.Featured, &item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt, &fav, &item.VisitCount, &item.LifecycleStatus, &item.PublishedAt, &item.PublishedBy, &item.ConfigVersion); err != nil {
			return nil, err
		}
		item.Favorite = fav == 1
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := r.loadRelations(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validPolicyType(value string) bool {
	switch value {
	case portaldomain.PolicyEveryone, portaldomain.PolicyUser, portaldomain.PolicyRole, portaldomain.PolicyGroup, portaldomain.PolicyOrganization:
		return true
	default:
		return false
	}
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (r *PortalRepo) GetIdentityBinding(ctx context.Context, orgID, appID string) (portaldomain.IdentityBinding, error) {
	var b portaldomain.IdentityBinding
	var redirectJSON string
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,application_id,provider_key,protocol,provider_application_ref,public_client_id,issuer,redirect_uris_json,configuration_status,verification_status,verified_at,verified_by,verification_error,config_version,created_at,updated_at FROM portal_application_identity_bindings WHERE organization_id=? AND application_id=?`), orgID, appID).Scan(&b.ID, &b.OrganizationID, &b.ApplicationID, &b.ProviderKey, &b.Protocol, &b.ProviderApplicationRef, &b.PublicClientID, &b.Issuer, &redirectJSON, &b.ConfigurationStatus, &b.VerificationStatus, &b.VerifiedAt, &b.VerifiedBy, &b.VerificationError, &b.ConfigVersion, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return portaldomain.IdentityBinding{}, err
	}
	b.RedirectURIs = portaldomain.DecodeRedirectURIs(redirectJSON)
	return b, nil
}

func (r *PortalRepo) UpsertIdentityBinding(ctx context.Context, orgID, actorID, appID string, input portaldomain.IdentityBindingInput, expectedVersion int64) (portaldomain.IdentityBinding, error) {
	if err := input.Validate(); err != nil {
		return portaldomain.IdentityBinding{}, err
	}
	now := time.Now().UTC()
	var out portaldomain.IdentityBinding
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		var id string
		var version int64
		readErr := r.db.QueryRowContext(txCtx, r.db.Rebind(`SELECT id,config_version FROM portal_application_identity_bindings WHERE organization_id=? AND application_id=?`), orgID, appID).Scan(&id, &version)
		if readErr != nil && readErr != sql.ErrNoRows {
			return readErr
		}
		if readErr == sql.ErrNoRows {
			id = uuid.NewString()
			if _, err := r.db.ExecContext(txCtx, r.db.Rebind(`INSERT INTO portal_application_identity_bindings(id,organization_id,application_id,provider_key,protocol,provider_application_ref,public_client_id,issuer,redirect_uris_json,configuration_status,verification_status,config_version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), id, orgID, appID, strings.ToLower(strings.TrimSpace(input.ProviderKey)), strings.ToUpper(strings.TrimSpace(input.Protocol)), strings.TrimSpace(input.ProviderApplicationRef), strings.TrimSpace(input.PublicClientID), strings.TrimSpace(input.Issuer), (&portaldomain.IdentityBinding{RedirectURIs: input.RedirectURIs}).RedirectURIsJSON(), "CONFIGURED", "PENDING", 1, now, now); err != nil {
				return err
			}
		} else {
			if expectedVersion > 0 && expectedVersion != version {
				return portaldomain.ErrOptimisticConflict
			}
			res, err := r.db.ExecContext(txCtx, r.db.Rebind(`UPDATE portal_application_identity_bindings SET provider_key=?,protocol=?,provider_application_ref=?,public_client_id=?,issuer=?,redirect_uris_json=?,configuration_status=?,verification_status='PENDING',verified_at=NULL,verified_by='',verification_error='',config_version=config_version+1,updated_at=? WHERE organization_id=? AND application_id=? AND config_version=?`), strings.ToLower(strings.TrimSpace(input.ProviderKey)), strings.ToUpper(strings.TrimSpace(input.Protocol)), strings.TrimSpace(input.ProviderApplicationRef), strings.TrimSpace(input.PublicClientID), strings.TrimSpace(input.Issuer), (&portaldomain.IdentityBinding{RedirectURIs: input.RedirectURIs}).RedirectURIsJSON(), "CONFIGURED", now, orgID, appID, version)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil || n != 1 {
				if err != nil {
					return err
				}
				return portaldomain.ErrOptimisticConflict
			}
		}
		_, err := r.db.ExecContext(txCtx, r.db.Rebind(`UPDATE portal_applications SET lifecycle_status=?,status=?,updated_by=?,updated_at=?,config_version=config_version+1 WHERE organization_id=? AND id=?`), portaldomain.LifecycleVerificationPending, portaldomain.StatusDisabled, actorID, now, orgID, appID)
		if err != nil {
			return err
		}
		out, err = r.GetIdentityBinding(txCtx, orgID, appID)
		return err
	})
	return out, err
}

func (r *PortalRepo) SetApplicationLifecycle(ctx context.Context, orgID, actorID, appID, lifecycle, status string, expectedVersion int64, publish bool) (portaldomain.Application, error) {
	now := time.Now().UTC()
	var item portaldomain.Application
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		query := `UPDATE portal_applications SET lifecycle_status=?,status=?,updated_by=?,updated_at=?,config_version=config_version+1`
		args := []any{lifecycle, status, actorID, now}
		if publish {
			query += `,published_at=?,published_by=?`
			args = append(args, now, actorID)
		}
		query += ` WHERE organization_id=? AND id=?`
		args = append(args, orgID, appID)
		if expectedVersion > 0 {
			query += ` AND config_version=?`
			args = append(args, expectedVersion)
		}
		res, err := r.db.ExecContext(txCtx, r.db.Rebind(query), args...)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			if expectedVersion > 0 {
				return portaldomain.ErrOptimisticConflict
			}
			return sql.ErrNoRows
		}
		item, err = r.GetApplication(txCtx, orgID, actorID, appID, true)
		return err
	})
	return item, err
}

func (r *PortalRepo) RecordIdentityVerification(ctx context.Context, orgID, actorID, appID string, passed bool, checkType, errorCode, evidence, requestID string, expectedVersion int64) (portaldomain.IdentityBinding, portaldomain.Verification, error) {
	now := time.Now().UTC()
	result := portaldomain.VerificationFailed
	if passed {
		result = portaldomain.VerificationPassed
	}
	var verification portaldomain.Verification
	err := r.db.WithinTx(ctx, func(txCtx context.Context) error {
		var b portaldomain.IdentityBinding
		var redirectJSON string
		var version int64
		if err := r.db.QueryRowContext(txCtx, r.db.Rebind(`SELECT id,organization_id,application_id,provider_key,protocol,provider_application_ref,public_client_id,issuer,redirect_uris_json,configuration_status,verification_status,verified_at,verified_by,verification_error,config_version,created_at,updated_at FROM portal_application_identity_bindings WHERE organization_id=? AND application_id=? FOR UPDATE`), orgID, appID).Scan(&b.ID, &b.OrganizationID, &b.ApplicationID, &b.ProviderKey, &b.Protocol, &b.ProviderApplicationRef, &b.PublicClientID, &b.Issuer, &redirectJSON, &b.ConfigurationStatus, &b.VerificationStatus, &b.VerifiedAt, &b.VerifiedBy, &b.VerificationError, &version, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return err
		}
		if expectedVersion > 0 && version != expectedVersion {
			return portaldomain.ErrOptimisticConflict
		}
		b.ConfigVersion = version
		b.RedirectURIs = portaldomain.DecodeRedirectURIs(redirectJSON)
		verification = portaldomain.Verification{ID: uuid.NewString(), OrganizationID: orgID, ApplicationID: appID, BindingID: b.ID, CheckType: checkType, Result: result, ErrorCode: errorCode, Evidence: evidence, VerifiedBy: actorID, OccurredAt: now, RequestID: requestID}
		if _, err := r.db.ExecContext(txCtx, r.db.Rebind(`INSERT INTO portal_application_verifications(id,organization_id,application_id,binding_id,check_type,result,error_code,evidence_json,verified_by,occurred_at,request_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)`), verification.ID, orgID, appID, b.ID, checkType, result, errorCode, evidence, actorID, now, requestID); err != nil {
			return err
		}
		status := portaldomain.VerificationFailed
		lifecycle := portaldomain.LifecycleVerificationFailed
		if passed {
			status = portaldomain.VerificationPassed
			lifecycle = portaldomain.LifecycleReady
		}
		_, err := r.db.ExecContext(txCtx, r.db.Rebind(`UPDATE portal_application_identity_bindings SET verification_status=?,verified_at=?,verified_by=?,verification_error=?,config_version=config_version+1,updated_at=? WHERE organization_id=? AND application_id=? AND config_version=?`), status, now, actorID, errorCode, now, orgID, appID, version)
		if err != nil {
			return err
		}
		_, err = r.db.ExecContext(txCtx, r.db.Rebind(`UPDATE portal_applications SET lifecycle_status=?,updated_by=?,updated_at=?,config_version=config_version+1 WHERE organization_id=? AND id=?`), lifecycle, actorID, now, orgID, appID)
		return err
	})
	if err != nil {
		return portaldomain.IdentityBinding{}, portaldomain.Verification{}, err
	}
	b, err := r.GetIdentityBinding(ctx, orgID, appID)
	return b, verification, err
}

func (r *PortalRepo) ListIdentityVerifications(ctx context.Context, orgID, appID string, limit int) ([]portaldomain.Verification, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,application_id,binding_id,check_type,result,error_code,evidence_json,verified_by,occurred_at,request_id FROM portal_application_verifications WHERE organization_id=? AND application_id=? ORDER BY occurred_at DESC LIMIT ?`), orgID, appID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]portaldomain.Verification, 0)
	for rows.Next() {
		var v portaldomain.Verification
		if err := rows.Scan(&v.ID, &v.OrganizationID, &v.ApplicationID, &v.BindingID, &v.CheckType, &v.Result, &v.ErrorCode, &v.Evidence, &v.VerifiedBy, &v.OccurredAt, &v.RequestID); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
