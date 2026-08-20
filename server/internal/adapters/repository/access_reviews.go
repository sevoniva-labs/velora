package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
)

func (r *IdentityRepo) CreateAccessReview(ctx context.Context, review domain.AccessReview) (domain.AccessReview, error) {
	review.ID = uuid.NewString()
	review.CreatedAt = review.CreatedAt.UTC()
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO access_reviews(id,organization_id,reviewer_id,status,due_at,created_by,created_at) VALUES(?,?,?,?,?,?,?)`), review.ID, review.OrganizationID, review.ReviewerID, review.Status, review.DueAt.UTC(), review.CreatedBy, review.CreatedAt); err != nil {
			return err
		}
		now := time.Now().UTC()
		rows, err := tx.QueryContext(ctx, r.db.Rebind(`SELECT user_id,login_name,role_key FROM (
			SELECT u.id AS user_id,u.login_name,r.role_key AS role_key FROM users u JOIN user_roles ur ON ur.user_id=u.id JOIN roles r ON r.id=ur.role_id WHERE u.organization_id=? AND u.status='ACTIVE'
			UNION
			SELECT u.id AS user_id,u.login_name,r.role_key AS role_key FROM users u JOIN user_group_members ugm ON ugm.user_id=u.id JOIN user_groups ug ON ug.id=ugm.group_id AND ug.status='ACTIVE' JOIN user_group_roles ugr ON ugr.group_id=ug.id JOIN roles r ON r.id=ugr.role_id WHERE u.organization_id=? AND u.status='ACTIVE'
			UNION
			SELECT u.id AS user_id,u.login_name,r.role_key AS role_key FROM users u JOIN temporary_role_grants trg ON trg.user_id=u.id AND trg.revoked_at IS NULL AND trg.valid_from<=? AND trg.valid_until>? JOIN roles r ON r.id=trg.role_id WHERE u.organization_id=? AND u.status='ACTIVE'
		) entitlement_snapshot ORDER BY login_name,role_key`), review.OrganizationID, review.OrganizationID, now, now, review.OrganizationID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var item domain.AccessReviewItem
			if err := rows.Scan(&item.UserID, &item.LoginName, &item.RoleKey); err != nil {
				return err
			}
			item.ID, item.ReviewID, item.OrganizationID, item.Decision, item.CreatedAt = uuid.NewString(), review.ID, review.OrganizationID, "PENDING", review.CreatedAt
			if _, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO access_review_items(id,review_id,organization_id,user_id,login_name,role_key,decision,created_at) VALUES(?,?,?,?,?,?,?,?)`), item.ID, item.ReviewID, item.OrganizationID, item.UserID, item.LoginName, item.RoleKey, item.Decision, item.CreatedAt); err != nil {
				return err
			}
		}
		return rows.Err()
	})
	if err != nil {
		return domain.AccessReview{}, err
	}
	return r.AccessReviewByID(ctx, review.OrganizationID, review.ID)
}

func (r *IdentityRepo) AccessReviewByID(ctx context.Context, organizationID, reviewID string) (domain.AccessReview, error) {
	var review domain.AccessReview
	var completed sql.NullTime
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT ar.id,ar.organization_id,ar.reviewer_id,u.login_name,ar.status,ar.due_at,ar.created_by,ar.created_at,ar.completed_at FROM access_reviews ar JOIN users u ON u.id=ar.reviewer_id WHERE ar.organization_id=? AND ar.id=?`), organizationID, reviewID).Scan(&review.ID, &review.OrganizationID, &review.ReviewerID, &review.ReviewerName, &review.Status, &review.DueAt, &review.CreatedBy, &review.CreatedAt, &completed)
	if completed.Valid {
		value := completed.Time
		review.CompletedAt = &value
	}
	return review, err
}

func (r *IdentityRepo) ListAccessReviews(ctx context.Context, organizationID string, limit int) ([]domain.AccessReview, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT ar.id,ar.organization_id,ar.reviewer_id,u.login_name,ar.status,ar.due_at,ar.created_by,ar.created_at,ar.completed_at FROM access_reviews ar JOIN users u ON u.id=ar.reviewer_id WHERE ar.organization_id=? ORDER BY ar.created_at DESC LIMIT ?`), organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.AccessReview, 0)
	for rows.Next() {
		var review domain.AccessReview
		var completed sql.NullTime
		if err := rows.Scan(&review.ID, &review.OrganizationID, &review.ReviewerID, &review.ReviewerName, &review.Status, &review.DueAt, &review.CreatedBy, &review.CreatedAt, &completed); err != nil {
			return nil, err
		}
		if completed.Valid {
			value := completed.Time
			review.CompletedAt = &value
		}
		items = append(items, review)
	}
	return items, rows.Err()
}

func (r *IdentityRepo) ListAccessReviewItems(ctx context.Context, organizationID, reviewID string) ([]domain.AccessReviewItem, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,review_id,organization_id,user_id,login_name,role_key,decision,reason,decided_by,decided_at,created_at FROM access_review_items WHERE organization_id=? AND review_id=? ORDER BY login_name,role_key`), organizationID, reviewID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]domain.AccessReviewItem, 0)
	for rows.Next() {
		var item domain.AccessReviewItem
		var decidedBy sql.NullString
		var decidedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.ReviewID, &item.OrganizationID, &item.UserID, &item.LoginName, &item.RoleKey, &item.Decision, &item.Reason, &decidedBy, &decidedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.DecidedBy = decidedBy.String
		if decidedAt.Valid {
			value := decidedAt.Time
			item.DecidedAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *IdentityRepo) DecideAccessReviewItem(ctx context.Context, organizationID, reviewID, itemID, reviewerID, decision, reason string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var status, assignedReviewer string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT ar.status,ar.reviewer_id FROM access_reviews ar WHERE ar.organization_id=? AND ar.id=? FOR UPDATE`), organizationID, reviewID).Scan(&status, &assignedReviewer); err != nil {
			return err
		}
		if status != domain.AccessReviewOpen || assignedReviewer != reviewerID {
			return sql.ErrNoRows
		}
		result, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE access_review_items SET decision=?,reason=?,decided_by=?,decided_at=? WHERE organization_id=? AND review_id=? AND id=? AND decision='PENDING'`), decision, reason, reviewerID, time.Now().UTC(), organizationID, reviewID, itemID)
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
		var pending int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM access_review_items WHERE organization_id=? AND review_id=? AND decision='PENDING'`), organizationID, reviewID).Scan(&pending); err != nil {
			return err
		}
		if pending == 0 {
			_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE access_reviews SET status=?,completed_at=? WHERE organization_id=? AND id=?`), domain.AccessReviewCompleted, time.Now().UTC(), organizationID, reviewID)
		}
		return err
	})
}
