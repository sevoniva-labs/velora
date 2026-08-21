package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/approval"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type ApprovalRepo struct{ db *database.DB }

func NewApprovalRepo(db *database.DB) *ApprovalRepo { return &ApprovalRepo{db: db} }

func (r *ApprovalRepo) Create(ctx context.Context, request domain.Request, approverIDs []string) (domain.Request, error) {
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		for _, userID := range append([]string{request.ApplicantID}, approverIDs...) {
			var count int
			if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM users WHERE id=? AND organization_id=? AND status='ACTIVE'`), userID, request.OrganizationID).Scan(&count); err != nil || count != 1 {
				if err != nil {
					return err
				}
				return sql.ErrNoRows
			}
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO approval_requests(id,organization_id,request_type,action,resource,resource_id,summary,payload_json,request_digest,applicant_id,approval_mode,required_approvals,status,expires_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), request.ID, request.OrganizationID, request.RequestType, request.Action, request.Resource, request.ResourceID, request.Summary, request.PayloadJSON, request.RequestDigest, request.ApplicantID, request.Mode, request.RequiredApprovals, request.Status, request.ExpiresAt, request.CreatedAt, request.UpdatedAt)
		if err != nil {
			return err
		}
		for _, assigneeID := range approverIDs {
			task := domain.Task{ID: uuid.NewString(), RequestID: request.ID, AssigneeID: assigneeID, Status: domain.StatusPending, CreatedAt: request.CreatedAt, UpdatedAt: request.CreatedAt}
			if _, err = tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO approval_tasks(id,request_id,assignee_id,status,decision,comment,transferred_from,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`), task.ID, task.RequestID, task.AssigneeID, task.Status, "", "", "", task.CreatedAt, task.UpdatedAt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.Request{}, err
	}
	return r.ByID(ctx, request.OrganizationID, request.ID)
}

func (r *ApprovalRepo) ByID(ctx context.Context, orgID, requestID string) (domain.Request, error) {
	now := time.Now().UTC()
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE approval_requests SET status='EXPIRED',updated_at=? WHERE id=? AND organization_id=? AND status='PENDING' AND expires_at<=?`), now, requestID, orgID, now)
	var request domain.Request
	err := r.db.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,request_type,action,resource,resource_id,summary,payload_json,request_digest,applicant_id,approval_mode,required_approvals,status,expires_at,created_at,updated_at FROM approval_requests WHERE id=? AND organization_id=?`), requestID, orgID).Scan(
		&request.ID, &request.OrganizationID, &request.RequestType, &request.Action, &request.Resource, &request.ResourceID, &request.Summary, &request.PayloadJSON, &request.RequestDigest, &request.ApplicantID, &request.Mode, &request.RequiredApprovals, &request.Status, &request.ExpiresAt, &request.CreatedAt, &request.UpdatedAt,
	)
	if err != nil {
		return domain.Request{}, err
	}
	request.Tasks, err = r.tasks(ctx, requestID)
	return request, err
}

func (r *ApprovalRepo) List(ctx context.Context, orgID, userID string, organizationWide bool, limit int) ([]domain.Request, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	now := time.Now().UTC()
	_, _ = r.db.ExecContext(ctx, r.db.Rebind(`UPDATE approval_requests SET status='EXPIRED',updated_at=? WHERE organization_id=? AND status='PENDING' AND expires_at<=?`), now, orgID, now)
	query := `SELECT DISTINCT ar.id,ar.organization_id,ar.request_type,ar.action,ar.resource,ar.resource_id,ar.summary,ar.payload_json,ar.request_digest,ar.applicant_id,ar.approval_mode,ar.required_approvals,ar.status,ar.expires_at,ar.created_at,ar.updated_at FROM approval_requests ar`
	args := []any{orgID}
	if !organizationWide {
		query += ` LEFT JOIN approval_tasks at ON at.request_id=ar.id WHERE ar.organization_id=? AND (ar.applicant_id=? OR at.assignee_id=? OR at.transferred_from=?)`
		args = append(args, userID, userID, userID)
	} else {
		query += ` WHERE ar.organization_id=?`
	}
	query += ` ORDER BY ar.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Request, 0)
	for rows.Next() {
		var request domain.Request
		if err := rows.Scan(&request.ID, &request.OrganizationID, &request.RequestType, &request.Action, &request.Resource, &request.ResourceID, &request.Summary, &request.PayloadJSON, &request.RequestDigest, &request.ApplicantID, &request.Mode, &request.RequiredApprovals, &request.Status, &request.ExpiresAt, &request.CreatedAt, &request.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, request)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range out {
		out[index].Tasks, err = r.tasks(ctx, out[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *ApprovalRepo) tasks(ctx context.Context, requestID string) ([]domain.Task, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,request_id,assignee_id,status,decision,comment,transferred_from,decided_at,created_at,updated_at FROM approval_tasks WHERE request_id=? ORDER BY created_at,id`), requestID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.Task, 0)
	for rows.Next() {
		var task domain.Task
		var decided sql.NullTime
		if err := rows.Scan(&task.ID, &task.RequestID, &task.AssigneeID, &task.Status, &task.Decision, &task.Comment, &task.TransferredFrom, &decided, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		if decided.Valid {
			value := decided.Time
			task.DecidedAt = &value
		}
		out = append(out, task)
	}
	return out, rows.Err()
}

func (r *ApprovalRepo) Decide(ctx context.Context, orgID, requestID, actorID, decision, comment string) (domain.Request, error) {
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		request, err := r.lockRequest(ctx, tx, orgID, requestID)
		if err != nil {
			return err
		}
		if request.Status != domain.StatusPending || !request.ExpiresAt.After(time.Now().UTC()) {
			return domain.ErrNotPending
		}
		if request.ApplicantID == actorID {
			return domain.ErrMakerChecker
		}
		var taskID, status string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id,status FROM approval_tasks WHERE request_id=? AND assignee_id=? FOR UPDATE`), requestID, actorID).Scan(&taskID, &status); err != nil {
			return domain.ErrTaskNotAssigned
		}
		if status != domain.StatusPending {
			return domain.ErrNotPending
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE approval_tasks SET status=?,decision=?,comment=?,decided_at=?,updated_at=? WHERE id=?`), decision, decision, comment, now, now, taskID); err != nil {
			return err
		}
		var approved, rejected, pending int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT SUM(CASE WHEN decision='APPROVE' THEN 1 ELSE 0 END),SUM(CASE WHEN decision='REJECT' THEN 1 ELSE 0 END),SUM(CASE WHEN status='PENDING' THEN 1 ELSE 0 END) FROM approval_tasks WHERE request_id=?`), requestID).Scan(&approved, &rejected, &pending); err != nil {
			return err
		}
		request.Status = domain.ResolveStatus(request.RequiredApprovals, approved, rejected, pending)
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE approval_requests SET status=?,updated_at=? WHERE id=?`), request.Status, now, requestID); err != nil {
			return err
		}
		if request.Status != domain.StatusPending {
			_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE approval_tasks SET status='CANCELLED',updated_at=? WHERE request_id=? AND status='PENDING'`), now, requestID)
		}
		return err
	})
	if err != nil {
		return domain.Request{}, err
	}
	return r.ByID(ctx, orgID, requestID)
}

func (r *ApprovalRepo) Transfer(ctx context.Context, orgID, requestID, actorID, newAssigneeID, comment string) (domain.Request, error) {
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		request, err := r.lockRequest(ctx, tx, orgID, requestID)
		if err != nil {
			return err
		}
		if request.Status != domain.StatusPending || !request.ExpiresAt.After(time.Now().UTC()) {
			return domain.ErrNotPending
		}
		if newAssigneeID == request.ApplicantID {
			return domain.ErrMakerChecker
		}
		var count int
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM users WHERE id=? AND organization_id=? AND status='ACTIVE'`), newAssigneeID, orgID).Scan(&count); err != nil || count != 1 {
			if err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		var taskID, status string
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id,status FROM approval_tasks WHERE request_id=? AND assignee_id=? FOR UPDATE`), requestID, actorID).Scan(&taskID, &status); err != nil {
			return domain.ErrTaskNotAssigned
		}
		if status != domain.StatusPending {
			return domain.ErrNotPending
		}
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT COUNT(*) FROM approval_tasks WHERE request_id=? AND assignee_id=?`), requestID, newAssigneeID).Scan(&count); err != nil || count > 0 {
			if err != nil {
				return err
			}
			return domain.ErrInvalidRequest
		}
		now := time.Now().UTC()
		_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE approval_tasks SET assignee_id=?,transferred_from=?,comment=?,updated_at=? WHERE id=?`), newAssigneeID, actorID, comment, now, taskID)
		return err
	})
	if err != nil {
		return domain.Request{}, err
	}
	return r.ByID(ctx, orgID, requestID)
}

func (r *ApprovalRepo) Withdraw(ctx context.Context, orgID, requestID, actorID, comment string) (domain.Request, error) {
	err := r.db.WithTx(ctx, func(tx *sql.Tx) error {
		request, err := r.lockRequest(ctx, tx, orgID, requestID)
		if err != nil {
			return err
		}
		if request.ApplicantID != actorID {
			return sql.ErrNoRows
		}
		if request.Status != domain.StatusPending {
			return domain.ErrNotPending
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, r.db.Rebind(`UPDATE approval_requests SET status='WITHDRAWN',updated_at=? WHERE id=?`), now, requestID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE approval_tasks SET status='CANCELLED',comment=?,updated_at=? WHERE request_id=? AND status='PENDING'`), comment, now, requestID)
		return err
	})
	if err != nil {
		return domain.Request{}, err
	}
	return r.ByID(ctx, orgID, requestID)
}

func (r *ApprovalRepo) ClaimExecution(ctx context.Context, orgID, requestID, actorID, expectedDigest string) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var applicantID, status, actualDigest string
		var expiresAt time.Time
		if err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT applicant_id,status,request_digest,expires_at FROM approval_requests WHERE id=? AND organization_id=? FOR UPDATE`), requestID, orgID).Scan(&applicantID, &status, &actualDigest, &expiresAt); err != nil {
			return err
		}
		if applicantID != actorID {
			return domain.ErrMakerChecker
		}
		if status != domain.StatusApproved || !expiresAt.After(time.Now().UTC()) {
			return domain.ErrNotPending
		}
		if actualDigest != expectedDigest {
			return domain.ErrDigestMismatch
		}
		var executedBy, executedDigest string
		lookupErr := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT executed_by,request_digest FROM approval_executions WHERE request_id=?`), requestID).Scan(&executedBy, &executedDigest)
		if lookupErr == nil {
			if executedBy == actorID && executedDigest == expectedDigest {
				// An external side effect may have timed out after the execution
				// ticket was claimed. Reusing the exact ticket and digest is safe
				// because the provider operation is itself idempotent.
				return nil
			}
			return domain.ErrAlreadyExecuted
		}
		if lookupErr != sql.ErrNoRows {
			return lookupErr
		}
		_, err := tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO approval_executions(id,request_id,executed_by,request_digest,executed_at) VALUES(?,?,?,?,?)`), uuid.NewString(), requestID, actorID, expectedDigest, time.Now().UTC())
		return err
	})
}

func (r *ApprovalRepo) lockRequest(ctx context.Context, tx *sql.Tx, orgID, requestID string) (domain.Request, error) {
	var request domain.Request
	err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,applicant_id,required_approvals,status,expires_at FROM approval_requests WHERE id=? AND organization_id=? FOR UPDATE`), requestID, orgID).Scan(&request.ID, &request.OrganizationID, &request.ApplicantID, &request.RequiredApprovals, &request.Status, &request.ExpiresAt)
	return request, err
}

func IsApprovalParticipant(request domain.Request, userID string) bool {
	if request.ApplicantID == userID {
		return true
	}
	for _, task := range request.Tasks {
		if task.AssigneeID == userID || task.TransferredFrom == userID {
			return true
		}
	}
	return false
}
