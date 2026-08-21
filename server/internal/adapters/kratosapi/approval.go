package kratosapi

import (
	"context"
	"database/sql"
	"errors"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	identityapp "github.com/sevoniva-labs/velora/server/internal/app/identity"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/approval"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
)

type ApprovalService struct {
	forgev1.UnimplementedApprovalServiceServer
	approval *approval.Service
	audit    *audit.Writer
	db       *database.DB
}

func NewApprovalService(service *approval.Service, auditWriter *audit.Writer, db *database.DB) *ApprovalService {
	return &ApprovalService{approval: service, audit: auditWriter, db: db}
}

func (s *ApprovalService) CreateApproval(ctx context.Context, req *forgev1.CreateApprovalRequest) (*forgev1.CreateApprovalResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "approval.create", "approval", "", map[string]any{"request_type": req.GetRequestType(), "action": req.GetAction(), "resource": req.GetResource()})
	var created domain.Request
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.approval.Create(txCtx, principal, approval.CreateInput{RequestType: req.GetRequestType(), Action: req.GetAction(), Resource: req.GetResource(), ResourceID: req.GetResourceId(), Summary: req.GetSummary(), PayloadJSON: req.GetPayloadJson(), Mode: req.GetMode(), RequiredApprovals: int(req.GetRequiredApprovals()), ApproverIDs: req.GetApproverIds(), ExpiresIn: durationSeconds(req.GetExpiresInSeconds())})
		if createErr == nil {
			event.ResourceID = created.ID
			event.Details["request_digest"] = created.RequestDigest
		}
		return createErr
	})
	if err != nil {
		return nil, approvalServiceError(err)
	}
	return &forgev1.CreateApprovalResponse{Approval: approvalProto(created)}, nil
}

func (s *ApprovalService) GetApproval(ctx context.Context, req *forgev1.GetApprovalRequest) (*forgev1.GetApprovalResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	item, err := s.approval.Get(ctx, principal, req.GetApprovalId())
	if err != nil {
		return nil, approvalServiceError(err)
	}
	return &forgev1.GetApprovalResponse{Approval: approvalProto(item)}, nil
}

func (s *ApprovalService) ListApprovals(ctx context.Context, _ *forgev1.ListApprovalsRequest) (*forgev1.ListApprovalsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.approval.List(ctx, principal)
	if err != nil {
		return nil, approvalServiceError(err)
	}
	reply := &forgev1.ListApprovalsResponse{Approvals: make([]*forgev1.ApprovalRequest, 0, len(items))}
	for _, item := range items {
		reply.Approvals = append(reply.Approvals, approvalProto(item))
	}
	return reply, nil
}

func (s *ApprovalService) DecideApproval(ctx context.Context, req *forgev1.DecideApprovalRequest) (*forgev1.DecideApprovalResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "approval.decide", "approval", req.GetApprovalId(), map[string]any{"decision": req.GetDecision()})
	var item domain.Request
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var operationErr error
		item, operationErr = s.approval.Decide(txCtx, principal, req.GetApprovalId(), req.GetDecision(), req.GetComment())
		return operationErr
	})
	if err != nil {
		return nil, approvalServiceError(err)
	}
	return &forgev1.DecideApprovalResponse{Approval: approvalProto(item)}, nil
}

func (s *ApprovalService) TransferApproval(ctx context.Context, req *forgev1.TransferApprovalRequest) (*forgev1.TransferApprovalResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "approval.transfer", "approval", req.GetApprovalId(), map[string]any{"new_assignee_id": req.GetNewAssigneeId()})
	var item domain.Request
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var operationErr error
		item, operationErr = s.approval.Transfer(txCtx, principal, req.GetApprovalId(), req.GetNewAssigneeId(), req.GetComment())
		return operationErr
	})
	if err != nil {
		return nil, approvalServiceError(err)
	}
	return &forgev1.TransferApprovalResponse{Approval: approvalProto(item)}, nil
}

func (s *ApprovalService) WithdrawApproval(ctx context.Context, req *forgev1.WithdrawApprovalRequest) (*forgev1.WithdrawApprovalResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "approval.withdraw", "approval", req.GetApprovalId(), nil)
	var item domain.Request
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var operationErr error
		item, operationErr = s.approval.Withdraw(txCtx, principal, req.GetApprovalId(), req.GetComment())
		return operationErr
	})
	if err != nil {
		return nil, approvalServiceError(err)
	}
	return &forgev1.WithdrawApprovalResponse{Approval: approvalProto(item)}, nil
}

func (s *ApprovalService) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
	if s.db == nil || s.audit == nil {
		return errors.New("reliable audit is unavailable")
	}
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := operation(txCtx); err != nil {
			return err
		}
		return s.audit.Write(txCtx, *event)
	})
}

func approvalProto(request domain.Request) *forgev1.ApprovalRequest {
	out := &forgev1.ApprovalRequest{Id: request.ID, OrganizationId: request.OrganizationID, RequestType: request.RequestType, Action: request.Action, Resource: request.Resource, ResourceId: request.ResourceID, Summary: request.Summary, PayloadJson: request.PayloadJSON, RequestDigest: request.RequestDigest, ApplicantId: request.ApplicantID, Mode: request.Mode, RequiredApprovals: int32(request.RequiredApprovals), Status: request.Status, ExpiresAt: timestamp(request.ExpiresAt), CreatedAt: timestamp(request.CreatedAt), UpdatedAt: timestamp(request.UpdatedAt), Tasks: make([]*forgev1.ApprovalTask, 0, len(request.Tasks))} // #nosec G115 -- domain approval validation bounds RequiredApprovals to the approver set.
	for _, task := range request.Tasks {
		out.Tasks = append(out.Tasks, &forgev1.ApprovalTask{Id: task.ID, AssigneeId: task.AssigneeID, Status: task.Status, Decision: task.Decision, Comment: task.Comment, TransferredFrom: task.TransferredFrom, DecidedAt: optionalTimestamp(task.DecidedAt)})
	}
	return out
}

func approvalServiceError(err error) error {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return kratoserrors.NotFound("NOT_FOUND", "approval resource not found")
	case errors.Is(err, domain.ErrMakerChecker), errors.Is(err, domain.ErrTaskNotAssigned), errors.Is(err, approval.ErrAccessDenied):
		return kratoserrors.Forbidden("PERMISSION_DENIED", "approval operation is not permitted")
	case errors.Is(err, domain.ErrNotPending):
		return kratoserrors.Conflict("APPROVAL_NOT_PENDING", "approval request is not pending")
	case errors.Is(err, domain.ErrInvalidRequest):
		return kratoserrors.BadRequest("INVALID_ARGUMENT", "invalid approval request")
	case errors.Is(err, identityapp.ErrStepUpRequired), errors.Is(err, identityapp.ErrInteractiveSessionRequired):
		return serviceError(err)
	default:
		return internalError(err)
	}
}

func durationSeconds(value int64) time.Duration { return time.Duration(value) * time.Second }
