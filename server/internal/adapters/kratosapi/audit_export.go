package kratosapi

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
)

const (
	defaultAuditExportLimit = 1000
	maximumAuditExportLimit = 5000
)

func (s *PlatformService) ExportAuditLogs(ctx context.Context, req *forgev1.ExportAuditLogsRequest) (*forgev1.ExportAuditLogsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}

	format := strings.ToLower(strings.TrimSpace(req.GetFormat()))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		return nil, kerrors.BadRequest("INVALID_EXPORT_FORMAT", "export format must be json or csv")
	}

	limit := int(req.GetLimit())
	if limit == 0 {
		limit = defaultAuditExportLimit
	}
	if limit < 1 || limit > maximumAuditExportLimit {
		return nil, kerrors.BadRequest("INVALID_EXPORT_LIMIT", "export limit must be between 1 and 5000")
	}

	var content []byte
	payload, err := json.Marshal(map[string]any{"format": format, "limit": limit})
	if err != nil {
		return nil, internalError(err)
	}
	event := newAuditEvent(ctx, principal, "audit.export", "audit_log", "organization", map[string]any{"format": format, "limit": limit, "approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "AUDIT_LOG_EXPORT",
			Action:      "audit.export",
			Resource:    "audit_log",
			ResourceID:  "organization",
			PayloadJSON: string(payload),
		}); executionErr != nil {
			return executionErr
		}
		items, listErr := s.audit.List(txCtx, principal.OrganizationID, limit)
		if listErr != nil {
			return listErr
		}
		content, listErr = encodeAuditExport(items, format)
		return listErr
	})
	if err != nil {
		return nil, serviceError(err)
	}

	contentType := "application/json; charset=utf-8"
	if format == "csv" {
		contentType = "text/csv; charset=utf-8"
	}
	return &forgev1.ExportAuditLogsResponse{
		Content: content, ContentType: contentType,
		Filename: fmt.Sprintf("audit-logs-%s.%s", time.Now().UTC().Format("20060102T150405Z"), format),
	}, nil
}

func encodeAuditExport(items []audit.Event, format string) ([]byte, error) {
	if format == "json" {
		return json.Marshal(items)
	}
	return encodeAuditCSV(items)
}

func encodeAuditCSV(items []audit.Event) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{
		"id", "occurred_at", "request_id", "organization_id", "actor_id", "actor_name",
		"action", "resource_type", "resource_id", "result", "client_ip", "details",
	}); err != nil {
		return nil, err
	}
	for _, item := range items {
		details, err := json.Marshal(item.Details)
		if err != nil {
			return nil, err
		}
		record := []string{
			item.ID, item.OccurredAt.UTC().Format(time.RFC3339Nano), item.RequestID, item.OrganizationID,
			item.ActorID, item.ActorName, item.Action, item.ResourceType, item.ResourceID,
			item.Result, item.ClientIP, string(details),
		}
		for index := range record {
			record[index] = sanitizeCSVCell(record[index])
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sanitizeCSVCell(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}
