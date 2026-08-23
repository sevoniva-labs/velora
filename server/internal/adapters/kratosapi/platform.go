package kratosapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	appapproval "github.com/sevoniva-labs/velora/server/internal/app/approval"
	"github.com/sevoniva-labs/velora/server/internal/app/audit"
	appconfigchange "github.com/sevoniva-labs/velora/server/internal/app/configchange"
	appdatapolicy "github.com/sevoniva-labs/velora/server/internal/app/datapolicy"
	appidentity "github.com/sevoniva-labs/velora/server/internal/app/identity"
	appportal "github.com/sevoniva-labs/velora/server/internal/app/portal"
	domainapproval "github.com/sevoniva-labs/velora/server/internal/domain/approval"
	domain "github.com/sevoniva-labs/velora/server/internal/domain/identity"
	portaldomain "github.com/sevoniva-labs/velora/server/internal/domain/portal"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
	"github.com/sevoniva-labs/velora/server/internal/platform/casdooradmin"
	"github.com/sevoniva-labs/velora/server/internal/platform/database"
	"github.com/sevoniva-labs/velora/server/internal/platform/httpserver"
)

type PlatformService struct {
	forgev1.UnimplementedPlatformServiceServer
	identity     *appidentity.Service
	portal       *appportal.Service
	approval     *appapproval.Service
	configChange *appconfigchange.Service
	dataPolicy   *appdatapolicy.Service
	audit        *audit.Writer
	db           *database.DB
}

func NewPlatformService(identity *appidentity.Service, portal *appportal.Service, approval *appapproval.Service, configChange *appconfigchange.Service, dataPolicy *appdatapolicy.Service, auditWriter *audit.Writer, db *database.DB) *PlatformService {
	return &PlatformService{identity: identity, portal: portal, approval: approval, configChange: configChange, dataPolicy: dataPolicy, audit: auditWriter, db: db}
}

func (s *PlatformService) recomputeApplicationAccess(ctx context.Context, principal domain.Principal) error {
	if s.portal == nil {
		return errors.New("application access projection is unavailable")
	}
	return s.portal.RecomputeOrganizationAccess(ctx, principal)
}

func (s *PlatformService) CreateUser(ctx context.Context, req *forgev1.CreateUserRequest) (*forgev1.CreateUserResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var created domain.User
	event := newAuditEvent(ctx, principal, "user.create", "user", "", map[string]any{"login_name": req.GetLoginName(), "roles": req.GetRoles()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		if s.identity.ManagedIdentityEnabled() {
			created, createErr = s.identity.CreateManagedUser(txCtx, principal, principal.OrganizationID, req.GetLoginName(), req.GetDisplayName(), req.GetEmail(), req.GetPassword(), req.GetRoles())
		} else {
			created, createErr = s.identity.CreateUser(txCtx, principal, principal.OrganizationID, req.GetLoginName(), req.GetDisplayName(), req.GetPassword(), req.GetRoles())
		}
		if createErr == nil {
			for _, entitlement := range req.GetEntitlements() {
				created, createErr = s.identity.UpdateUserEntitlement(txCtx, principal, created.ID, entitlement.GetApplicationCode(), entitlement.GetStatus(), entitlement.GetRoles())
				if createErr != nil {
					break
				}
			}
		}
		if createErr == nil {
			event.ResourceID = created.ID
			createErr = s.recomputeApplicationAccess(txCtx, principal)
		}
		return createErr
	})
	if err != nil {
		if s.identity.ManagedIdentityEnabled() {
			s.identity.CompensateManagedUser(context.WithoutCancel(ctx), req.GetLoginName())
		}
		return nil, serviceError(err)
	}
	return &forgev1.CreateUserResponse{User: userProto(created)}, nil
}

func (s *PlatformService) ListDepartments(ctx context.Context, _ *forgev1.ListDepartmentsRequest) (*forgev1.ListDepartmentsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.identity.ListDepartments(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListDepartmentsResponse{Departments: make([]*forgev1.Department, 0, len(items))}
	for _, item := range items {
		reply.Departments = append(reply.Departments, departmentProto(item))
	}
	return reply, nil
}

func (s *PlatformService) CreateDepartment(ctx context.Context, req *forgev1.CreateDepartmentRequest) (*forgev1.CreateDepartmentResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sortOrder, err := checkedInt(req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	var created domain.Department
	event := newAuditEvent(ctx, principal, "department.create", "department", "", map[string]any{"department_key": req.GetDepartmentKey(), "parent_id": req.GetParentId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.identity.CreateDepartment(txCtx, principal, principal.OrganizationID, domain.Department{
			ParentID: req.GetParentId(), Key: req.GetDepartmentKey(), Name: req.GetName(), Status: req.GetStatus(), SortOrder: sortOrder,
		})
		if createErr == nil {
			event.ResourceID = created.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateDepartmentResponse{Department: departmentProto(created)}, nil
}

func (s *PlatformService) UpdateDepartment(ctx context.Context, req *forgev1.UpdateDepartmentRequest) (*forgev1.UpdateDepartmentResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sortOrder, err := checkedInt(req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	var updated domain.Department
	event := newAuditEvent(ctx, principal, "department.update", "department", req.GetDepartmentId(), map[string]any{"parent_id": req.GetParentId(), "status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdateDepartment(txCtx, principal, principal.OrganizationID, req.GetDepartmentId(), domain.Department{
			ParentID: req.GetParentId(), Name: req.GetName(), Status: req.GetStatus(), SortOrder: sortOrder,
		})
		if updateErr == nil {
			updateErr = s.recomputeApplicationAccess(txCtx, principal)
		}
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateDepartmentResponse{Department: departmentProto(updated)}, nil
}

func (s *PlatformService) ListPositions(ctx context.Context, _ *forgev1.ListPositionsRequest) (*forgev1.ListPositionsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.identity.ListPositions(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListPositionsResponse{Positions: make([]*forgev1.Position, 0, len(items))}
	for _, item := range items {
		reply.Positions = append(reply.Positions, positionProto(item))
	}
	return reply, nil
}

func (s *PlatformService) CreatePosition(ctx context.Context, req *forgev1.CreatePositionRequest) (*forgev1.CreatePositionResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sortOrder, err := checkedInt(req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	var created domain.Position
	event := newAuditEvent(ctx, principal, "position.create", "position", "", map[string]any{"position_key": req.GetPositionKey(), "department_id": req.GetDepartmentId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.identity.CreatePosition(txCtx, principal, principal.OrganizationID, domain.Position{
			DepartmentID: req.GetDepartmentId(), Key: req.GetPositionKey(), Name: req.GetName(), Description: req.GetDescription(), Status: req.GetStatus(), SortOrder: sortOrder,
		})
		if createErr == nil {
			event.ResourceID = created.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreatePositionResponse{Position: positionProto(created)}, nil
}

func (s *PlatformService) UpdatePosition(ctx context.Context, req *forgev1.UpdatePositionRequest) (*forgev1.UpdatePositionResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sortOrder, err := checkedInt(req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	var updated domain.Position
	event := newAuditEvent(ctx, principal, "position.update", "position", req.GetPositionId(), map[string]any{"department_id": req.GetDepartmentId(), "status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdatePosition(txCtx, principal, principal.OrganizationID, req.GetPositionId(), domain.Position{
			DepartmentID: req.GetDepartmentId(), Name: req.GetName(), Description: req.GetDescription(), Status: req.GetStatus(), SortOrder: sortOrder,
		})
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdatePositionResponse{Position: positionProto(updated)}, nil
}

func (s *PlatformService) ListUserGroups(ctx context.Context, _ *forgev1.ListUserGroupsRequest) (*forgev1.ListUserGroupsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.identity.ListUserGroups(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListUserGroupsResponse{UserGroups: make([]*forgev1.UserGroup, 0, len(items))}
	for _, item := range items {
		reply.UserGroups = append(reply.UserGroups, userGroupProto(item))
	}
	return reply, nil
}

func (s *PlatformService) CreateUserGroup(ctx context.Context, req *forgev1.CreateUserGroupRequest) (*forgev1.CreateUserGroupResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var created domain.UserGroup
	event := newAuditEvent(ctx, principal, "user_group.create", "user_group", "", map[string]any{"group_key": req.GetGroupKey()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.identity.CreateUserGroup(txCtx, principal, principal.OrganizationID, domain.UserGroup{Key: req.GetGroupKey(), Name: req.GetName(), Description: req.GetDescription(), Status: req.GetStatus()})
		if createErr == nil {
			event.ResourceID = created.ID
		}
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateUserGroupResponse{UserGroup: userGroupProto(created)}, nil
}

func (s *PlatformService) UpdateUserGroup(ctx context.Context, req *forgev1.UpdateUserGroupRequest) (*forgev1.UpdateUserGroupResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var updated domain.UserGroup
	event := newAuditEvent(ctx, principal, "user_group.update", "user_group", req.GetGroupId(), map[string]any{"status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdateUserGroup(txCtx, principal, principal.OrganizationID, req.GetGroupId(), domain.UserGroup{Name: req.GetName(), Description: req.GetDescription(), Status: req.GetStatus()})
		if updateErr == nil {
			updateErr = s.recomputeApplicationAccess(txCtx, principal)
		}
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserGroupResponse{UserGroup: userGroupProto(updated)}, nil
}

func (s *PlatformService) UpdateUserGroupMembers(ctx context.Context, req *forgev1.UpdateUserGroupMembersRequest) (*forgev1.UpdateUserGroupMembersResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user_group.members.update", "user_group", req.GetGroupId(), map[string]any{"member_count": len(req.GetUserIds())})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.identity.UpdateUserGroupMembers(txCtx, principal, principal.OrganizationID, req.GetGroupId(), req.GetUserIds()); err != nil {
			return err
		}
		return s.recomputeApplicationAccess(txCtx, principal)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserGroupMembersResponse{}, nil
}

func (s *PlatformService) UpdateUserGroupRoles(ctx context.Context, req *forgev1.UpdateUserGroupRolesRequest) (*forgev1.UpdateUserGroupRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user_group.roles.update", "user_group", req.GetGroupId(), map[string]any{"roles": req.GetRoles()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.identity.UpdateUserGroupRoles(txCtx, principal, principal.OrganizationID, req.GetGroupId(), req.GetRoles()); err != nil {
			return err
		}
		return s.recomputeApplicationAccess(txCtx, principal)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserGroupRolesResponse{}, nil
}

func (s *PlatformService) ListUserAssignments(ctx context.Context, req *forgev1.ListUserAssignmentsRequest) (*forgev1.ListUserAssignmentsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.identity.ListUserAssignments(ctx, principal, req.GetUserId())
	if err != nil {
		return nil, serviceError(err)
	}
	reply := &forgev1.ListUserAssignmentsResponse{Assignments: make([]*forgev1.UserAssignment, 0, len(items))}
	for _, item := range items {
		reply.Assignments = append(reply.Assignments, userAssignmentProto(item))
	}
	return reply, nil
}

func (s *PlatformService) ReplaceUserAssignments(ctx context.Context, req *forgev1.ReplaceUserAssignmentsRequest) (*forgev1.ReplaceUserAssignmentsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	assignments := make([]domain.UserAssignment, 0, len(req.GetAssignments()))
	for _, item := range req.GetAssignments() {
		assignment := domain.UserAssignment{DepartmentID: item.GetDepartmentId(), PositionID: item.GetPositionId(), Primary: item.GetPrimary()}
		if item.GetValidFrom() != nil {
			assignment.ValidFrom = item.GetValidFrom().AsTime()
		}
		if item.GetValidUntil() != nil {
			until := item.GetValidUntil().AsTime()
			assignment.ValidUntil = &until
		}
		assignments = append(assignments, assignment)
	}
	event := newAuditEvent(ctx, principal, "user.assignments.replace", "user", req.GetUserId(), map[string]any{"assignment_count": len(assignments)})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.identity.ReplaceUserAssignments(txCtx, principal, principal.OrganizationID, req.GetUserId(), assignments); err != nil {
			return err
		}
		return s.recomputeApplicationAccess(txCtx, principal)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ReplaceUserAssignmentsResponse{}, nil
}

func (s *PlatformService) ListUserEffectiveApplicationAccess(ctx context.Context, req *forgev1.ListUserEffectiveApplicationAccessRequest) (*forgev1.ListUserEffectiveApplicationAccessResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.portal.ListUserEffectiveApplicationAccess(ctx, principal, req.GetUserId())
	if err != nil {
		return nil, serviceError(err)
	}
	reply := &forgev1.ListUserEffectiveApplicationAccessResponse{Accesses: make([]*forgev1.UserEffectiveApplicationAccess, 0, len(items))}
	for _, item := range items {
		access := &forgev1.UserEffectiveApplicationAccess{UserId: item.UserID, ApplicationId: item.ApplicationID, ApplicationCode: item.ApplicationCode, ApplicationName: item.ApplicationName, Roles: item.Roles, Status: item.Status}
		for _, itemSource := range item.Sources {
			access.Sources = append(access.Sources, &forgev1.EffectiveApplicationAccessSource{GrantId: itemSource.GrantID, SubjectType: itemSource.SubjectType, SubjectId: itemSource.SubjectID, SubjectName: itemSource.SubjectName, Effect: itemSource.Effect})
		}
		reply.Accesses = append(reply.Accesses, access)
	}
	return reply, nil
}

func (s *PlatformService) UpdateOrganization(ctx context.Context, req *forgev1.UpdateOrganizationRequest) (*forgev1.UpdateOrganizationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	maxUsers, err := checkedInt(req.GetMaxUsers())
	if err != nil {
		return nil, err
	}
	maxSessions, err := checkedInt(req.GetMaxActiveSessions())
	if err != nil {
		return nil, err
	}
	var updated domain.Organization
	event := newAuditEvent(ctx, principal, "organization.update", "organization", principal.OrganizationID, map[string]any{"status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.identity.UpdateOrganization(txCtx, principal.OrganizationID, domain.Organization{
			Name: req.GetName(), Description: req.GetDescription(), Status: req.GetStatus(), MaxUsers: maxUsers, MaxSessions: maxSessions,
		})
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateOrganizationResponse{Organization: organizationProto(updated)}, nil
}

func (s *PlatformService) UpdateSecurityPolicy(ctx context.Context, req *forgev1.UpdateSecurityPolicyRequest) (*forgev1.UpdateSecurityPolicyResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := securityPolicyDomain(req.GetPolicy())
	if err != nil {
		return nil, err
	}
	policyPayload, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(req.GetPolicy())
	if err != nil {
		return nil, internalError(err)
	}
	var updated domain.SecurityPolicy
	event := newAuditEvent(ctx, principal, "security.config.update", "security", "policy", map[string]any{"approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "SECURITY_POLICY_CHANGE",
			Action:      "security.config.update",
			Resource:    "security",
			ResourceID:  "policy",
			PayloadJSON: string(policyPayload),
		}); executionErr != nil {
			return executionErr
		}
		var updateErr error
		updated, updateErr = s.identity.UpdateSecurityPolicy(txCtx, principal.OrganizationID, principal.UserID, policy)
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateSecurityPolicyResponse{Policy: securityPolicyProto(updated)}, nil
}

func (s *PlatformService) UpdateRolePermissions(ctx context.Context, req *forgev1.UpdateRolePermissionsRequest) (*forgev1.UpdateRolePermissionsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	permissions, payload, err := stringSetPayload("permissions", req.GetPermissions())
	if err != nil {
		return nil, serviceError(err)
	}
	event := newAuditEvent(ctx, principal, "role.permissions.update", "role", req.GetRoleKey(), map[string]any{"permissions": permissions, "approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "ROLE_PERMISSION_CHANGE",
			Action:      "role.permissions.update",
			Resource:    "role",
			ResourceID:  req.GetRoleKey(),
			PayloadJSON: payload,
		}); executionErr != nil {
			return executionErr
		}
		return s.identity.UpdateRolePermissions(txCtx, principal, principal.OrganizationID, req.GetRoleKey(), permissions)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateRolePermissionsResponse{Role: &forgev1.Role{Key: req.GetRoleKey(), Permissions: permissions}}, nil
}

func (s *PlatformService) UpdateRoleDataScope(ctx context.Context, req *forgev1.UpdateRoleDataScopeRequest) (*forgev1.UpdateRoleDataScopeResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	departmentIDs, payload, err := roleDataScopePayload(req.GetDataScope(), req.GetDepartmentIds())
	if err != nil {
		return nil, serviceError(err)
	}
	event := newAuditEvent(ctx, principal, "role.data_scope.update", "role", req.GetRoleKey(), map[string]any{"data_scope": req.GetDataScope(), "department_count": len(departmentIDs), "approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "ROLE_DATA_SCOPE_CHANGE",
			Action:      "role.data_scope.update",
			Resource:    "role",
			ResourceID:  req.GetRoleKey(),
			PayloadJSON: payload,
		}); executionErr != nil {
			return executionErr
		}
		return s.identity.UpdateRoleDataScope(txCtx, principal, principal.OrganizationID, req.GetRoleKey(), req.GetDataScope(), departmentIDs)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateRoleDataScopeResponse{Role: &forgev1.Role{Key: req.GetRoleKey(), DataScope: req.GetDataScope(), DataScopeDepartmentIds: departmentIDs}}, nil
}

func (s *PlatformService) UpdateUserRoles(ctx context.Context, req *forgev1.UpdateUserRolesRequest) (*forgev1.UpdateUserRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	roles, payload, err := userRoleChangePayload(req.GetRoles())
	if err != nil {
		return nil, serviceError(err)
	}
	event := newAuditEvent(ctx, principal, "user.roles.update", "user", req.GetUserId(), map[string]any{"roles": roles, "approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "USER_ROLE_CHANGE",
			Action:      "user.roles.update",
			Resource:    "user",
			ResourceID:  req.GetUserId(),
			PayloadJSON: payload,
		}); executionErr != nil {
			return executionErr
		}
		if err := s.identity.UpdateUserRoles(txCtx, principal, principal.OrganizationID, req.GetUserId(), roles); err != nil {
			return err
		}
		return s.recomputeApplicationAccess(txCtx, principal)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserRolesResponse{User: &forgev1.User{Id: req.GetUserId(), OrganizationId: principal.OrganizationID, Roles: roles}}, nil
}

func (s *PlatformService) UpdateUserStatus(ctx context.Context, req *forgev1.UpdateUserStatusRequest) (*forgev1.UpdateUserStatusResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.status.update", "user", req.GetUserId(), map[string]any{"status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if err := s.identity.SetManagedUserStatus(txCtx, principal, req.GetUserId(), req.GetStatus()); err != nil {
			return err
		}
		return s.recomputeApplicationAccess(txCtx, principal)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateUserStatusResponse{User: &forgev1.User{Id: req.GetUserId(), OrganizationId: principal.OrganizationID, Status: req.GetStatus()}}, nil
}

func (s *PlatformService) UpdateUserEntitlement(ctx context.Context, req *forgev1.UpdateUserEntitlementRequest) (*forgev1.UpdateUserEntitlementResponse, error) {
	if _, err := requiredPrincipal(ctx); err != nil {
		return nil, err
	}
	return nil, kratoserrors.BadRequest("INVALID_ARGUMENT", "application access must be managed through access rules")
}

func (s *PlatformService) UnlockUser(ctx context.Context, req *forgev1.UnlockUserRequest) (*forgev1.UnlockUserResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.unlock", "user", req.GetUserId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.UnlockUser(txCtx, principal.OrganizationID, req.GetUserId())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UnlockUserResponse{User: &forgev1.User{Id: req.GetUserId(), OrganizationId: principal.OrganizationID}}, nil
}

func (s *PlatformService) ResetUserPassword(ctx context.Context, req *forgev1.ResetUserPasswordRequest) (*forgev1.ResetUserPasswordResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "user.password.reset", "user", req.GetUserId(), map[string]any{"force_change": true, "approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "USER_PASSWORD_RESET",
			Action:      "user.password.reset",
			Resource:    "user",
			ResourceID:  req.GetUserId(),
			PayloadJSON: `{"force_change":true}`,
		}); executionErr != nil {
			return executionErr
		}
		return s.identity.AdminResetPassword(txCtx, principal.OrganizationID, req.GetUserId(), req.GetPassword())
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.ResetUserPasswordResponse{}, nil
}

func (s *PlatformService) RevokeSession(ctx context.Context, req *forgev1.RevokeSessionRequest) (*forgev1.RevokeSessionResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	event := newAuditEvent(ctx, principal, "session.revoke", "session", req.GetSessionId(), nil)
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		return s.identity.RevokeSession(txCtx, principal.OrganizationID, req.GetSessionId(), principal.SessionID)
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.RevokeSessionResponse{}, nil
}

func (s *PlatformService) ListUsers(ctx context.Context, req *forgev1.ListUsersRequest) (*forgev1.ListUsersResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	page := int(req.GetPage())
	pageSize := int(req.GetPageSize())
	if pageSize == 0 && req.GetLimit() > 0 {
		pageSize = int(req.GetLimit())
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	users, total, err := s.identity.ListUsersPage(ctx, principal, page, pageSize, req.GetKeyword(), req.GetStatus(), req.GetRoleKey())
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListUsersResponse{Users: make([]*forgev1.User, 0, len(users)), Total: total, Page: int32(page), PageSize: int32(pageSize)} // #nosec G115 -- page values are bounded above.
	for _, user := range users {
		reply.Users = append(reply.Users, userProto(user))
	}
	return reply, nil
}

func (s *PlatformService) GetOrganization(ctx context.Context, _ *forgev1.GetOrganizationRequest) (*forgev1.GetOrganizationResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	organization, err := s.identity.Organization(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.GetOrganizationResponse{Organization: organizationProto(organization)}, nil
}

func (s *PlatformService) GetSecurityPolicy(ctx context.Context, _ *forgev1.GetSecurityPolicyRequest) (*forgev1.GetSecurityPolicyResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := s.identity.SecurityPolicy(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	return &forgev1.GetSecurityPolicyResponse{Policy: securityPolicyProto(policy)}, nil
}

func (s *PlatformService) ListRoles(ctx context.Context, _ *forgev1.ListRolesRequest) (*forgev1.ListRolesResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.identity.ListRoles(ctx, principal.OrganizationID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListRolesResponse{Roles: make([]*forgev1.Role, 0, len(roles))}
	for _, role := range roles {
		reply.Roles = append(reply.Roles, roleProto(role))
	}
	return reply, nil
}

func (s *PlatformService) CreateRole(ctx context.Context, req *forgev1.CreateRoleRequest) (*forgev1.CreateRoleResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var role domain.Role
	event := newAuditEvent(ctx, principal, "role.create", "role", req.GetRoleKey(), map[string]any{"name": req.GetName()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var createErr error
		role, createErr = s.identity.CreateRole(txCtx, principal, principal.OrganizationID, req.GetRoleKey(), req.GetName(), req.GetDescription())
		return createErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CreateRoleResponse{Role: roleProto(role)}, nil
}

func (s *PlatformService) UpdateRole(ctx context.Context, req *forgev1.UpdateRoleRequest) (*forgev1.UpdateRoleResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var role domain.Role
	event := newAuditEvent(ctx, principal, "role.update", "role", req.GetRoleKey(), map[string]any{"name": req.GetName(), "status": req.GetStatus()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var updateErr error
		role, updateErr = s.identity.UpdateRole(txCtx, principal, principal.OrganizationID, req.GetRoleKey(), req.GetName(), req.GetDescription(), req.GetStatus())
		if updateErr == nil {
			updateErr = s.recomputeApplicationAccess(txCtx, principal)
		}
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateRoleResponse{Role: roleProto(role)}, nil
}

func (s *PlatformService) CopyRole(ctx context.Context, req *forgev1.CopyRoleRequest) (*forgev1.CopyRoleResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	var role domain.Role
	event := newAuditEvent(ctx, principal, "role.copy", "role", req.GetRoleKey(), map[string]any{"source_role_key": req.GetSourceRoleKey(), "name": req.GetName()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		var copyErr error
		role, copyErr = s.identity.CopyRole(txCtx, principal, principal.OrganizationID, req.GetSourceRoleKey(), req.GetRoleKey(), req.GetName(), req.GetDescription())
		return copyErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.CopyRoleResponse{Role: roleProto(role)}, nil
}

func (s *PlatformService) ListPermissions(ctx context.Context, _ *forgev1.ListPermissionsRequest) (*forgev1.ListPermissionsResponse, error) {
	permissions, err := s.identity.ListPermissions(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListPermissionsResponse{Permissions: make([]*forgev1.Permission, 0, len(permissions))}
	for _, permission := range permissions {
		reply.Permissions = append(reply.Permissions, permissionProto(permission))
	}
	return reply, nil
}

func (s *PlatformService) ListMenus(ctx context.Context, _ *forgev1.ListMenusRequest) (*forgev1.ListMenusResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	menus, err := s.identity.ListMenus(ctx, principal)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListMenusResponse{Menus: make([]*forgev1.Menu, 0, len(menus))}
	for _, menu := range menus {
		reply.Menus = append(reply.Menus, menuProto(menu))
	}
	return reply, nil
}

func (s *PlatformService) UpdateMenu(ctx context.Context, req *forgev1.UpdateMenuRequest) (*forgev1.UpdateMenuResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"menu_key": req.GetMenuKey(), "parent_key": req.GetParentKey(), "name": req.GetName(), "route": req.GetRoute(),
		"icon": req.GetIcon(), "permission_key": req.GetPermissionKey(), "sort_order": req.GetSortOrder(), "status": req.GetStatus(),
	})
	if err != nil {
		return nil, internalError(err)
	}
	var updated domain.Menu
	event := newAuditEvent(ctx, principal, "menu.update", "menu", req.GetMenuKey(), map[string]any{"approval_id": req.GetApprovalId()})
	err = s.audited(ctx, event, func(txCtx context.Context) error {
		if s.approval == nil {
			return appapproval.ErrApprovalRequired
		}
		if executionErr := s.approval.AuthorizeExecution(txCtx, principal, req.GetApprovalId(), appapproval.ExecutionInput{
			RequestType: "MENU_CHANGE", Action: "menu.update", Resource: "menu", ResourceID: req.GetMenuKey(), PayloadJSON: string(payload),
		}); executionErr != nil {
			return executionErr
		}
		var updateErr error
		updated, updateErr = s.identity.UpdateMenu(txCtx, principal, principal.OrganizationID, req.GetMenuKey(), domain.Menu{
			ParentKey: req.GetParentKey(), Name: req.GetName(), Route: req.GetRoute(), Icon: req.GetIcon(), PermissionKey: req.GetPermissionKey(), SortOrder: int(req.GetSortOrder()), Status: req.GetStatus(),
		})
		return updateErr
	})
	if err != nil {
		return nil, serviceError(err)
	}
	return &forgev1.UpdateMenuResponse{Menu: menuProto(updated)}, nil
}

func (s *PlatformService) ListSessions(ctx context.Context, _ *forgev1.ListSessionsRequest) (*forgev1.ListSessionsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := s.identity.ListSessions(ctx, principal.OrganizationID, principal.SessionID)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListSessionsResponse{Sessions: make([]*forgev1.Session, 0, len(sessions))}
	for _, session := range sessions {
		reply.Sessions = append(reply.Sessions, sessionProto(session))
	}
	return reply, nil
}

func (s *PlatformService) ListAuditLogs(ctx context.Context, req *forgev1.ListAuditLogsRequest) (*forgev1.ListAuditLogsResponse, error) {
	principal, err := requiredPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	limit := int(req.GetLimit())
	events, err := s.audit.List(ctx, principal.OrganizationID, limit)
	if err != nil {
		return nil, internalError(err)
	}
	reply := &forgev1.ListAuditLogsResponse{Events: make([]*forgev1.AuditEvent, 0, len(events))}
	for _, event := range events {
		reply.Events = append(reply.Events, auditEventProto(event))
	}
	return reply, nil
}

func requiredPrincipal(ctx context.Context) (domain.Principal, error) {
	principal, ok := authn.Principal(ctx)
	if !ok || principal.OrganizationID == "" {
		return domain.Principal{}, kratoserrors.Unauthorized("UNAUTHENTICATED", "authenticated organization is required")
	}
	return principal, nil
}

func (s *PlatformService) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
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

func newAuditEvent(ctx context.Context, principal domain.Principal, action, resourceType, resourceID string, details map[string]any) *audit.Event {
	event := &audit.Event{
		OrganizationID: principal.OrganizationID, ActorID: principal.UserID, ActorName: principal.LoginName,
		Action: action, ResourceType: resourceType, ResourceID: resourceID, Details: details,
	}
	if tr, ok := transport.FromServerContext(ctx); ok {
		event.RequestID = tr.RequestHeader().Get("X-Request-ID")
	}
	if ip := httpserver.ClientIP(ctx); ip != "" {
		event.ClientIP = ip
	}
	if remote, ok := peer.FromContext(ctx); ok && remote.Addr != nil {
		if event.ClientIP != "" {
			return event
		}
		host, _, err := net.SplitHostPort(remote.Addr.String())
		if err == nil {
			event.ClientIP = host
		}
	}
	return event
}

func checkedInt(value int64) (int, error) {
	if value < 0 || (strconv.IntSize == 32 && value > math.MaxInt32) {
		return 0, kratoserrors.BadRequest("INVALID_ARGUMENT", "numeric value is out of range")
	}
	return int(value), nil // #nosec G115 -- guarded above on 32-bit; int64 and int have equal width on 64-bit.
}

func securityPolicyDomain(policy *forgev1.SecurityPolicy) (domain.SecurityPolicy, error) {
	if policy == nil {
		return domain.SecurityPolicy{}, kratoserrors.BadRequest("INVALID_ARGUMENT", "policy is required")
	}
	minLength, err := checkedInt(policy.GetPasswordMinLength())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	history, err := checkedInt(policy.GetPasswordHistory())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	maxAge, err := checkedInt(policy.GetPasswordMaxAgeDays())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	maxFailures, err := checkedInt(policy.GetLoginMaxFailures())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	maxSessions, err := checkedInt(policy.GetMaxActiveSessions())
	if err != nil {
		return domain.SecurityPolicy{}, err
	}
	return domain.SecurityPolicy{
		PasswordMinLength: minLength, PasswordRequireUpper: policy.GetPasswordRequireUpper(),
		PasswordRequireLower: policy.GetPasswordRequireLower(), PasswordRequireDigit: policy.GetPasswordRequireDigit(),
		PasswordRequireSymbol: policy.GetPasswordRequireSymbol(), PasswordHistory: history, PasswordMaxAgeDays: maxAge,
		LoginMaxFailures: maxFailures, LoginLockDurationSeconds: policy.GetLoginLockDurationSeconds(),
		SessionTTLSeconds: policy.GetSessionTtlSeconds(), MaxConcurrentSessions: maxSessions,
	}, nil
}

func serviceError(err error) error {
	switch {
	case errors.Is(err, appportal.ErrNotFound):
		return kratoserrors.NotFound("NOT_FOUND", "portal resource not found")
	case errors.Is(err, appportal.ErrAccessDenied):
		return kratoserrors.Forbidden("PERMISSION_DENIED", "portal access denied")
	case errors.Is(err, appportal.ErrInvalid), errors.Is(err, appportal.ErrDisabled), errors.Is(err, portaldomain.ErrInvalidApplication), errors.Is(err, portaldomain.ErrInvalidLaunchURL):
		return kratoserrors.BadRequest("INVALID_ARGUMENT", "portal request violates policy")
	case errors.Is(err, portaldomain.ErrInvalidIdentityBinding), errors.Is(err, portaldomain.ErrIdentityBindingRequired):
		return kratoserrors.BadRequest("INVALID_IDENTITY_BINDING", "identity binding violates policy")
	case errors.Is(err, portaldomain.ErrPublishNotReady):
		return kratoserrors.BadRequest("PUBLISH_NOT_READY", "application has not passed identity verification")
	case errors.Is(err, portaldomain.ErrOptimisticConflict):
		return kratoserrors.Conflict("CONFIG_VERSION_CONFLICT", "configuration was changed by another operator")
	case errors.Is(err, casdooradmin.ErrApprovalRequired):
		return kratoserrors.BadRequest("APPROVAL_REQUIRED", "maker-checker approval is required")
	case errors.Is(err, sql.ErrNoRows):
		return kratoserrors.NotFound("NOT_FOUND", "resource not found")
	case errors.Is(err, appidentity.ErrGrantCeiling), errors.Is(err, appidentity.ErrLastSystemAdmin):
		return kratoserrors.Forbidden("PERMISSION_DENIED", "operation is not permitted")
	case errors.Is(err, appidentity.ErrRoleConflict):
		return kratoserrors.Conflict("ROLE_CONFLICT", "role combination violates segregation of duties")
	case errors.Is(err, appidentity.ErrInteractiveSessionRequired):
		return kratoserrors.Forbidden("INTERACTIVE_SESSION_REQUIRED", "interactive session is required")
	case errors.Is(err, appidentity.ErrStepUpRequired):
		return kratoserrors.Forbidden("STEP_UP_REQUIRED", "recent multi-factor authentication is required")
	case errors.Is(err, appapproval.ErrApprovalRequired):
		return kratoserrors.BadRequest("APPROVAL_REQUIRED", "approved execution ticket is required")
	case errors.Is(err, domainapproval.ErrDigestMismatch):
		return kratoserrors.Conflict("APPROVAL_DIGEST_MISMATCH", "approval does not authorize this operation")
	case errors.Is(err, domainapproval.ErrAlreadyExecuted):
		return kratoserrors.Conflict("APPROVAL_ALREADY_EXECUTED", "approval execution ticket has already been used")
	case errors.Is(err, domainapproval.ErrNotPending):
		return kratoserrors.Conflict("APPROVAL_NOT_EXECUTABLE", "approval is not executable")
	case errors.Is(err, domainapproval.ErrMakerChecker), errors.Is(err, appapproval.ErrAccessDenied):
		return kratoserrors.Forbidden("APPROVAL_ACCESS_DENIED", "approval execution is not permitted")
	case errors.Is(err, audit.ErrIntegrityViolation):
		return kratoserrors.Conflict("AUDIT_INTEGRITY_FAILED", "audit log integrity verification failed")
	case errors.Is(err, appidentity.ErrInvalidCredentials):
		return kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication failed")
	case errors.Is(err, appidentity.ErrInvalidMFA):
		return kratoserrors.Unauthorized("INVALID_MFA", "invalid multi-factor authentication code")
	case errors.Is(err, appidentity.ErrMFAAlreadyEnabled):
		return kratoserrors.Conflict("MFA_ALREADY_ENABLED", "multi-factor authentication is already enabled")
	case errors.Is(err, appidentity.ErrMFANotPending):
		return kratoserrors.BadRequest("MFA_ENROLLMENT_NOT_PENDING", "multi-factor authentication enrollment is not pending")
	case errors.Is(err, appidentity.ErrInvalidRole), errors.Is(err, appidentity.ErrInvalidLoginName),
		errors.Is(err, appidentity.ErrPasswordPolicy), errors.Is(err, appidentity.ErrPasswordReused),
		errors.Is(err, appidentity.ErrInvalidSecurityPolicy), errors.Is(err, appidentity.ErrInvalidDepartment),
		errors.Is(err, appidentity.ErrInvalidPosition), errors.Is(err, appidentity.ErrInvalidUserGroup),
		errors.Is(err, appidentity.ErrInvalidUserAssignment), errors.Is(err, appidentity.ErrInvalidDataScope), errors.Is(err, appidentity.ErrInvalidMenu):
		return kratoserrors.BadRequest("INVALID_ARGUMENT", "request violates policy")
	case errors.Is(err, appidentity.ErrInvalidFederatedIdentity):
		return kratoserrors.BadRequest("INVALID_FEDERATED_IDENTITY", "external identity mapping violates policy")
	case errors.Is(err, appidentity.ErrInvalidAccessReview):
		return kratoserrors.BadRequest("INVALID_ACCESS_REVIEW", "access review violates policy")
	case errors.Is(err, appidentity.ErrInvalidTemporaryGrant):
		return kratoserrors.BadRequest("INVALID_TEMPORARY_GRANT", "temporary role grant violates policy")
	default:
		return internalError(err)
	}
}

func userRoleChangePayload(roleKeys []string) ([]string, string, error) {
	return stringSetPayload("roles", roleKeys)
}

func stringSetPayload(key string, values []string) ([]string, string, error) {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			unique[value] = struct{}{}
		}
	}
	normalized := make([]string, 0, len(unique))
	for value := range unique {
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	payload, err := json.Marshal(map[string]any{key: normalized})
	if err != nil {
		return nil, "", err
	}
	return normalized, string(payload), nil
}

func roleDataScopePayload(dataScope string, departmentIDs []string) ([]string, string, error) {
	departments, _, err := stringSetPayload("department_ids", departmentIDs)
	if err != nil {
		return nil, "", err
	}
	payload, err := json.Marshal(map[string]any{"data_scope": strings.TrimSpace(dataScope), "department_ids": departments})
	if err != nil {
		return nil, "", err
	}
	return departments, string(payload), nil
}

func internalError(error) error {
	return kratoserrors.InternalServer("INTERNAL", "operation failed")
}

func timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func optionalTimestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamp(*value)
}

func userProto(user domain.User) *forgev1.User {
	out := &forgev1.User{
		Id: user.ID, OrganizationId: user.OrganizationID, LoginName: user.LoginName,
		DisplayName: user.DisplayName, Email: user.Email, IdentitySource: user.IdentitySource, Status: user.Status, MustChangePassword: user.MustChangePassword,
		LockedUntil: optionalTimestamp(user.LockedUntil), PasswordChangedAt: timestamp(user.PasswordChangedAt),
		CreatedAt: timestamp(user.CreatedAt), Roles: user.Roles, Permissions: user.Permissions,
	}
	for _, entitlement := range user.Entitlements {
		out.Entitlements = append(out.Entitlements, &forgev1.ApplicationEntitlement{ApplicationCode: entitlement.ApplicationCode, ApplicationName: entitlement.ApplicationName, Status: entitlement.Status, Roles: entitlement.Roles, Version: entitlement.Version})
	}
	return out
}

func organizationProto(organization domain.Organization) *forgev1.Organization {
	return &forgev1.Organization{
		Id: organization.ID, OrganizationKey: organization.Key, Name: organization.Name,
		Description: organization.Description, Status: organization.Status, MaxUsers: int64(organization.MaxUsers),
		MaxActiveSessions: int64(organization.MaxSessions), CreatedAt: timestamp(organization.CreatedAt),
		UpdatedAt: timestamp(organization.UpdatedAt),
	}
}

func departmentProto(department domain.Department) *forgev1.Department {
	return &forgev1.Department{
		Id: department.ID, OrganizationId: department.OrganizationID, ParentId: department.ParentID,
		DepartmentKey: department.Key, Name: department.Name, Status: department.Status,
		SortOrder: int64(department.SortOrder), CreatedAt: timestamp(department.CreatedAt), UpdatedAt: timestamp(department.UpdatedAt),
	}
}

func positionProto(position domain.Position) *forgev1.Position {
	return &forgev1.Position{
		Id: position.ID, OrganizationId: position.OrganizationID, DepartmentId: position.DepartmentID,
		PositionKey: position.Key, Name: position.Name, Description: position.Description, Status: position.Status,
		SortOrder: int64(position.SortOrder), CreatedAt: timestamp(position.CreatedAt), UpdatedAt: timestamp(position.UpdatedAt),
	}
}

func userGroupProto(group domain.UserGroup) *forgev1.UserGroup {
	return &forgev1.UserGroup{
		Id: group.ID, OrganizationId: group.OrganizationID, GroupKey: group.Key, Name: group.Name,
		Description: group.Description, Status: group.Status, Roles: group.Roles, MemberIds: group.MemberIDs,
		MemberCount: int64(group.MemberCount), CreatedAt: timestamp(group.CreatedAt), UpdatedAt: timestamp(group.UpdatedAt),
	}
}

func userAssignmentProto(assignment domain.UserAssignment) *forgev1.UserAssignment {
	return &forgev1.UserAssignment{
		Id: assignment.ID, OrganizationId: assignment.OrganizationID, UserId: assignment.UserID,
		DepartmentId: assignment.DepartmentID, PositionId: assignment.PositionID, Primary: assignment.Primary,
		ValidFrom: timestamp(assignment.ValidFrom), ValidUntil: optionalTimestamp(assignment.ValidUntil), CreatedAt: timestamp(assignment.CreatedAt),
	}
}

func securityPolicyProto(policy domain.SecurityPolicy) *forgev1.SecurityPolicy {
	return &forgev1.SecurityPolicy{
		PasswordMinLength: int64(policy.PasswordMinLength), PasswordRequireUpper: policy.PasswordRequireUpper,
		PasswordRequireLower: policy.PasswordRequireLower, PasswordRequireDigit: policy.PasswordRequireDigit,
		PasswordRequireSymbol: policy.PasswordRequireSymbol, PasswordHistory: int64(policy.PasswordHistory),
		PasswordMaxAgeDays: int64(policy.PasswordMaxAgeDays), LoginMaxFailures: int64(policy.LoginMaxFailures),
		LoginLockDurationSeconds: policy.LoginLockDurationSeconds, SessionTtlSeconds: policy.SessionTTLSeconds,
		MaxActiveSessions: int64(policy.MaxConcurrentSessions),
	}
}

func permissionProto(permission domain.Permission) *forgev1.Permission {
	return &forgev1.Permission{Key: permission.Key, Name: permission.Name}
}

func menuProto(menu domain.Menu) *forgev1.Menu {
	return &forgev1.Menu{Id: menu.ID, OrganizationId: menu.OrganizationID, Key: menu.Key, ParentKey: menu.ParentKey, Name: menu.Name, Route: menu.Route, Icon: menu.Icon, PermissionKey: menu.PermissionKey, SortOrder: int32(menu.SortOrder), Status: menu.Status} // #nosec G115 -- menus.sort_order is persisted as SQL INTEGER and bounded by the menu mutation policy.
}

func roleProto(role domain.Role) *forgev1.Role {
	permissions := make([]string, 0, len(role.Permissions))
	for _, permission := range role.Permissions {
		permissions = append(permissions, permission.Key)
	}
	return &forgev1.Role{Key: role.Key, Name: role.Name, Description: role.Description, Status: role.Status, DataScope: role.DataScope, DataScopeDepartmentIds: role.Departments, Permissions: permissions}
}

func sessionProto(session domain.Session) *forgev1.Session {
	return &forgev1.Session{
		Id: session.ID, UserId: session.UserID, LoginName: session.LoginName,
		ClientIp: session.ClientIP, UserAgent: session.UserAgent, CreatedAt: timestamp(session.CreatedAt),
		ExpiresAt: timestamp(session.ExpiresAt), LastSeenAt: timestamp(session.LastSeenAt), Current: session.Current,
	}
}

func auditEventProto(event audit.Event) *forgev1.AuditEvent {
	details, _ := json.Marshal(event.Details)
	return &forgev1.AuditEvent{
		Id: event.ID, OccurredAt: timestamp(event.OccurredAt), RequestId: event.RequestID,
		OrganizationId: event.OrganizationID, ActorId: event.ActorID, ActorName: event.ActorName,
		Action: event.Action, ResourceType: event.ResourceType, ResourceId: event.ResourceID,
		Result: event.Result, ClientIp: event.ClientIP, DetailsJson: string(details), SequenceNo: event.SequenceNo, PrevHash: event.PrevHash, EventHash: event.EventHash,
	}
}
