package kratosapi

import (
	"context"
	"encoding/json"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/platform/security/datapolicy"
)

func (s *PlatformService) ListDataFieldPolicies(ctx context.Context, _ *forgev1.ListDataFieldPoliciesRequest) (*forgev1.ListDataFieldPoliciesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.dataPolicy == nil {
		return nil, kratoserrors.InternalServer("DATA_POLICY_UNAVAILABLE", "data policy service is unavailable")
	}
	items, err := s.dataPolicy.List(ctx, principal)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListDataFieldPoliciesResponse{Policies: make([]*forgev1.DataFieldPolicy, 0, len(items))}
	for _, item := range items {
		reply.Policies = append(reply.Policies, dataFieldPolicyProto(item))
	}
	return reply, nil
}

func (s *PlatformService) UpsertDataFieldPolicy(ctx context.Context, req *forgev1.UpsertDataFieldPolicyRequest) (*forgev1.UpsertDataFieldPolicyResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.dataPolicy == nil || s.approval == nil {
		return nil, kratoserrors.InternalServer("DATA_POLICY_UNAVAILABLE", "data policy approval boundary is unavailable")
	}
	input := req.GetPolicy()
	if input == nil || strings.TrimSpace(input.GetFieldKey()) == "" {
		return nil, kratoserrors.BadRequest("INVALID_DATA_POLICY", "data policy field_key is required")
	}
	policy := datapolicy.FieldPolicy{
		Key: input.GetFieldKey(), Classification: datapolicy.Classification(input.GetClassification()), Owner: input.GetOwner(), Purpose: input.GetPurpose(), Residency: input.GetResidency(),
		RetentionDays: int(input.GetRetentionDays()), Tags: input.GetTags(), Mask: datapolicy.MaskStrategy(input.GetMaskStrategy()), ExportApproval: input.GetExportApproval(), Watermark: input.GetWatermark(),
	}
	if err := s.dataPolicy.Validate(policy); err != nil {
		return nil, serviceError(err)
	}
	payload, err := protojson.Marshal(input)
	if err != nil {
		return nil, internalError(err)
	}
	var saved datapolicy.Record
	event := newAuditEvent(ctx, principal, "data_policy.upsert", "data_field_policy", input.GetFieldKey(), map[string]any{
		"approval_id": req.GetApprovalId(), "classification": input.GetClassification(), "mask_strategy": input.GetMaskStrategy(),
	})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "DATA_POLICY_CHANGE", Action: "data_policy.upsert", Resource: "data_field_policy", ResourceID: input.GetFieldKey(), PayloadJSON: string(payload),
		}); err != nil {
			return err
		}
		var saveErr error
		saved, saveErr = s.dataPolicy.Upsert(txCtx, principal, policy)
		return saveErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpsertDataFieldPolicyResponse{Policy: dataFieldPolicyProto(saved)}, nil
}

func (s *PlatformService) AuthorizeDataExport(ctx context.Context, req *forgev1.AuthorizeDataExportRequest) (*forgev1.AuthorizeDataExportResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.dataPolicy == nil || s.approval == nil {
		return nil, kratoserrors.InternalServer("DATA_POLICY_UNAVAILABLE", "data policy approval boundary is unavailable")
	}
	payload, err := json.Marshal(map[string]any{"field_keys": req.GetFieldKeys(), "purpose": req.GetPurpose(), "watermark": req.GetWatermark()})
	if err != nil {
		return nil, internalError(err)
	}
	event := newAuditEvent(ctx, principal, "data_export.authorize", "data_export", strings.Join(req.GetFieldKeys(), ","), map[string]any{
		"approval_id": req.GetApprovalId(), "purpose": req.GetPurpose(), "field_count": len(req.GetFieldKeys()),
	})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.dataPolicy.AuthorizeExport(txCtx, principal, req.GetFieldKeys(), datapolicy.ExportRequest{ApprovalID: req.GetApprovalId(), Purpose: req.GetPurpose(), Watermark: req.GetWatermark()}); err != nil {
			return err
		}
		if err := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "DATA_EXPORT", Action: "data.export", Resource: "data_field_policy", ResourceID: strings.Join(req.GetFieldKeys(), ","), PayloadJSON: string(payload),
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.AuthorizeDataExportResponse{Authorized: true, Purpose: req.GetPurpose(), Watermark: req.GetWatermark()}, nil
}

func (s *PlatformService) ListDataDeletionEvidence(ctx context.Context, _ *forgev1.ListDataDeletionEvidenceRequest) (*forgev1.ListDataDeletionEvidenceResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.dataPolicy == nil {
		return nil, kratoserrors.InternalServer("DATA_RETENTION_UNAVAILABLE", "data retention service is unavailable")
	}
	items, err := s.dataPolicy.ListDeletionEvidence(ctx, principal)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListDataDeletionEvidenceResponse{Evidence: make([]*forgev1.DataDeletionEvidence, 0, len(items))}
	for _, item := range items {
		reply.Evidence = append(reply.Evidence, deletionEvidenceProto(item))
	}
	return reply, nil
}

func (s *PlatformService) RecordDataDeletionEvidence(ctx context.Context, req *forgev1.RecordDataDeletionEvidenceRequest) (*forgev1.RecordDataDeletionEvidenceResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if s.dataPolicy == nil || s.approval == nil {
		return nil, kratoserrors.InternalServer("DATA_RETENTION_UNAVAILABLE", "data retention approval boundary is unavailable")
	}
	if req.GetDeletedAt() == nil {
		return nil, kratoserrors.BadRequest("INVALID_RETENTION_EVIDENCE", "deleted_at is required")
	}
	evidence := datapolicy.DeletionEvidence{
		ResourceType: req.GetResourceType(), ResourceDigest: req.GetResourceDigest(), FieldKeys: req.GetFieldKeys(), Reason: req.GetReason(), RecordsDeleted: req.GetRecordsDeleted(), DeletedAt: req.GetDeletedAt().AsTime().UTC(),
	}
	if err := s.dataPolicy.ValidateDeletionEvidence(ctx, principal, evidence); err != nil {
		return nil, serviceError(err)
	}
	payload, err := json.Marshal(map[string]any{
		"resource_type": evidence.ResourceType, "resource_digest": evidence.ResourceDigest, "field_keys": evidence.FieldKeys, "reason": evidence.Reason, "records_deleted": evidence.RecordsDeleted, "deleted_at": evidence.DeletedAt,
	})
	if err != nil {
		return nil, internalError(err)
	}
	var recorded datapolicy.DeletionEvidence
	event := newAuditEvent(ctx, principal, "data_retention.deletion_evidence.record", "data_deletion_evidence", evidence.ResourceDigest, map[string]any{
		"approval_id": req.GetApprovalId(), "field_count": len(evidence.FieldKeys), "records_deleted": evidence.RecordsDeleted,
	})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "DATA_RETENTION", Action: "data_retention.deletion_evidence.record", Resource: "data_deletion_evidence", ResourceID: evidence.ResourceDigest, PayloadJSON: string(payload),
		}); err != nil {
			return err
		}
		var recordErr error
		recorded, recordErr = s.dataPolicy.RecordDeletionEvidence(txCtx, principal, evidence)
		return recordErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RecordDataDeletionEvidenceResponse{Evidence: deletionEvidenceProto(recorded)}, nil
}

func dataFieldPolicyProto(item datapolicy.Record) *forgev1.DataFieldPolicy {
	return &forgev1.DataFieldPolicy{
		Id: item.ID, OrganizationId: item.OrganizationID, FieldKey: item.Key, Classification: string(item.Classification), Owner: item.Owner, Purpose: item.Purpose, Residency: item.Residency,
		RetentionDays: int64(item.RetentionDays), Tags: item.Tags, MaskStrategy: string(item.Mask), ExportApproval: item.ExportApproval, Watermark: item.Watermark,
		CreatedAt: timestamp(item.CreatedAt), UpdatedAt: timestamp(item.UpdatedAt),
	}
}

func deletionEvidenceProto(item datapolicy.DeletionEvidence) *forgev1.DataDeletionEvidence {
	return &forgev1.DataDeletionEvidence{
		Id: item.ID, OrganizationId: item.OrganizationID, ResourceType: item.ResourceType, ResourceDigest: item.ResourceDigest, FieldKeys: item.FieldKeys, Reason: item.Reason, RecordsDeleted: item.RecordsDeleted,
		DeletedAt: timestamppb.New(item.DeletedAt), EvidenceHash: item.EvidenceHash, CreatedAt: timestamppb.New(item.CreatedAt),
	}
}
