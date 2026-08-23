package kratosapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	appconfigchange "github.com/sevoniva-labs/velora/server/internal/app/configchange"
	platformconfig "github.com/sevoniva-labs/velora/server/internal/platform/configchange"
)

func (s *PlatformService) ListConfigChanges(ctx context.Context, _ *forgev1.ListConfigChangesRequest) (*forgev1.ListConfigChangesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.configChange == nil {
		return nil, kratoserrors.InternalServer("CONFIG_CHANGE_UNAVAILABLE", "config change service is unavailable")
	}
	items, err := s.configChange.List(ctx, principal)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListConfigChangesResponse{Changes: make([]*forgev1.ConfigChange, 0, len(items))}
	for _, item := range items {
		reply.Changes = append(reply.Changes, configChangeProto(item))
	}
	return reply, nil
}

func (s *PlatformService) CreateConfigChange(ctx context.Context, req *forgev1.CreateConfigChangeRequest) (*forgev1.CreateConfigChangeResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.configChange == nil {
		return nil, kratoserrors.InternalServer("CONFIG_CHANGE_UNAVAILABLE", "config change service is unavailable")
	}
	input := appconfigchange.CreateInput{
		Namespace: req.GetNamespace(), Group: req.GetGroup(), DataID: req.GetDataId(), Version: req.GetVersion(),
		ExpectedPreviousVersion: req.GetExpectedPreviousVersion(), ValueDigest: req.GetValueDigest(), ValueRef: req.GetValueRef(), Sensitive: req.GetSensitive(),
	}
	var created platformconfig.Change
	event := newAuditEvent(ctx, principal, "config_change.create", "config_change", req.GetDataId(), map[string]any{"version": req.GetVersion(), "sensitive": req.GetSensitive()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.configChange.Create(txCtx, principal, input)
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateConfigChangeResponse{Change: configChangeProto(created)}, nil
}

func (s *PlatformService) ApproveConfigChange(ctx context.Context, req *forgev1.ApproveConfigChangeRequest) (*forgev1.ApproveConfigChangeResponse, error) {
	change, err := s.transitionConfigChange(ctx, req.GetChangeId(), req.GetApprovalId(), platformconfig.ActionApprove, "CONFIG_CHANGE_APPROVE", "config_change.approve")
	if err != nil {
		return nil, err
	}
	return &forgev1.ApproveConfigChangeResponse{Change: configChangeProto(change)}, nil
}

func (s *PlatformService) PublishConfigChange(ctx context.Context, req *forgev1.PublishConfigChangeRequest) (*forgev1.PublishConfigChangeResponse, error) {
	change, err := s.transitionConfigChange(ctx, req.GetChangeId(), req.GetApprovalId(), platformconfig.ActionPublish, "CONFIG_CHANGE_PUBLISH", "config_change.publish")
	if err != nil {
		return nil, err
	}
	return &forgev1.PublishConfigChangeResponse{Change: configChangeProto(change)}, nil
}

func (s *PlatformService) RequestConfigRollback(ctx context.Context, req *forgev1.RequestConfigRollbackRequest) (*forgev1.RequestConfigRollbackResponse, error) {
	change, err := s.transitionConfigChange(ctx, req.GetChangeId(), req.GetApprovalId(), platformconfig.ActionRequestRollback, "CONFIG_CHANGE_ROLLBACK_REQUEST", "config_change.rollback.request")
	if err != nil {
		return nil, err
	}
	return &forgev1.RequestConfigRollbackResponse{Change: configChangeProto(change)}, nil
}

func (s *PlatformService) RollbackConfigChange(ctx context.Context, req *forgev1.RollbackConfigChangeRequest) (*forgev1.RollbackConfigChangeResponse, error) {
	change, err := s.transitionConfigChange(ctx, req.GetChangeId(), req.GetApprovalId(), platformconfig.ActionRollback, "CONFIG_CHANGE_ROLLBACK", "config_change.rollback")
	if err != nil {
		return nil, err
	}
	return &forgev1.RollbackConfigChangeResponse{Change: configChangeProto(change)}, nil
}

func (s *PlatformService) transitionConfigChange(ctx context.Context, id, approvalID string, action platformconfig.Action, requestType, auditAction string) (platformconfig.Change, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return platformconfig.Change{}, err
	}
	if s.configChange == nil || s.approval == nil {
		return platformconfig.Change{}, kratoserrors.InternalServer("CONFIG_CHANGE_UNAVAILABLE", "config change approval boundary is unavailable")
	}
	id = strings.TrimSpace(id)
	approvalID = strings.TrimSpace(approvalID)
	if id == "" || approvalID == "" {
		return platformconfig.Change{}, kratoserrors.BadRequest("CONFIG_CHANGE_APPROVAL_REQUIRED", "change_id and approval_id are required")
	}
	payload, err := json.Marshal(map[string]string{"change_id": id, "action": string(action)})
	if err != nil {
		return platformconfig.Change{}, internalError(err)
	}
	var updated platformconfig.Change
	event := newAuditEvent(ctx, principal, auditAction, "config_change", id, map[string]any{"approval_id": approvalID})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.approval.AuthorizeExecution(txCtx, principal, approvalID, appapproval.ExecutionInput{RequestType: requestType, Action: auditAction, Resource: "config_change", ResourceID: id, PayloadJSON: string(payload)}); err != nil {
			return err
		}
		var transitionErr error
		updated, _, transitionErr = s.configChange.Transition(txCtx, principal, id, platformconfig.Request{Action: action, At: time.Now().UTC(), ActorID: principal.UserID, ApprovalID: approvalID})
		return transitionErr
	})
	if err != nil {
		return platformconfig.Change{}, serviceError(err)
	}
	return updated, nil
}

func configChangeProto(item platformconfig.Change) *forgev1.ConfigChange {
	return &forgev1.ConfigChange{
		Id: item.ID, OrganizationId: item.OrganizationID, Namespace: item.Namespace, Group: item.Group, DataId: item.DataID,
		Version: item.Version, ExpectedPreviousVersion: item.ExpectedPreviousVersion, ValueDigest: item.ValueDigest, ValueRef: item.ValueRef,
		Sensitive: item.Sensitive, CreatedBy: item.CreatedBy, ApprovedBy: item.ApprovedBy, ApprovalId: item.ApprovalID, State: string(item.State), UpdatedAt: timestamp(item.UpdatedAt),
	}
}
