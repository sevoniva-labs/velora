package authz

import (
	"context"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
)

const platformOperationPrefix = "/forge.v1.PlatformService/"
const approvalOperationPrefix = "/forge.v1.ApprovalService/"
const portalOperationPrefix = "/forge.v1.PortalService/"

func Server(rules map[string][]string) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "transport context is required")
			}
			// The enrollment token is the single-use bearer credential for this
			// exact operation. It cannot authorize any other Portal API.
			if tr.Operation() == forgev1.OperationPortalServiceConsumeApplicationEnrollment {
				return next(ctx, req)
			}
			principal, authenticated := authn.Principal(ctx)
			if authenticated && principal.MustChangePassword && !allowedBeforePasswordChange(tr.Operation()) {
				return nil, kratoserrors.Forbidden("PASSWORD_CHANGE_REQUIRED", "password change is required")
			}
			required, registered := rules[tr.Operation()]
			if !registered {
				if strings.HasPrefix(tr.Operation(), platformOperationPrefix) || strings.HasPrefix(tr.Operation(), approvalOperationPrefix) || strings.HasPrefix(tr.Operation(), portalOperationPrefix) {
					return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "operation has no authorization policy")
				}
				return next(ctx, req)
			}
			// A registered operation with no explicit permissions is available to
			// every authenticated principal. This is used by the end-user portal:
			// application visibility is enforced by its access-policy layer, so a
			// user with no matching applications receives an empty result instead
			// of a misleading authorization failure.
			if !authenticated || (len(required) > 0 && !principal.HasPermission(required...)) {
				return nil, kratoserrors.Forbidden("PERMISSION_DENIED", "permission denied")
			}
			return next(ctx, req)
		}
	}
}

func allowedBeforePasswordChange(operation string) bool {
	switch operation {
	case forgev1.OperationIdentityServiceGetCurrentUser,
		forgev1.OperationIdentityServiceChangePassword,
		forgev1.OperationIdentityServiceLogout:
		return true
	default:
		return false
	}
}

func PlatformRules() map[string][]string {
	return map[string][]string{
		forgev1.OperationPlatformServiceListUsers:                          {"system.user.read"},
		forgev1.OperationPlatformServiceCreateUser:                         {"system.user.create"},
		forgev1.OperationPlatformServiceListDepartments:                    {"system.department.read"},
		forgev1.OperationPlatformServiceCreateDepartment:                   {"system.department.manage"},
		forgev1.OperationPlatformServiceUpdateDepartment:                   {"system.department.manage"},
		forgev1.OperationPlatformServiceListPositions:                      {"system.position.read"},
		forgev1.OperationPlatformServiceCreatePosition:                     {"system.position.manage"},
		forgev1.OperationPlatformServiceUpdatePosition:                     {"system.position.manage"},
		forgev1.OperationPlatformServiceListUserGroups:                     {"system.user_group.read"},
		forgev1.OperationPlatformServiceCreateUserGroup:                    {"system.user_group.manage"},
		forgev1.OperationPlatformServiceUpdateUserGroup:                    {"system.user_group.manage"},
		forgev1.OperationPlatformServiceUpdateUserGroupMembers:             {"system.user_group.manage"},
		forgev1.OperationPlatformServiceUpdateUserGroupRoles:               {"system.user_group.manage"},
		forgev1.OperationPlatformServiceListUserAssignments:                {"system.user.assignment.read"},
		forgev1.OperationPlatformServiceReplaceUserAssignments:             {"system.user.assignment.manage"},
		forgev1.OperationPlatformServiceListUserEffectiveApplicationAccess: {"system.user.read"},
		forgev1.OperationPlatformServiceGetOrganization:                    {"system.organization.read"},
		forgev1.OperationPlatformServiceUpdateOrganization:                 {"system.organization.manage"},
		forgev1.OperationPlatformServiceGetSecurityPolicy:                  {"system.config.read"},
		forgev1.OperationPlatformServiceUpdateSecurityPolicy:               {"system.security.manage"},
		forgev1.OperationPlatformServiceListRoles:                          {"system.role.read"},
		forgev1.OperationPlatformServiceCreateRole:                         {"system.role.manage"},
		forgev1.OperationPlatformServiceUpdateRole:                         {"system.role.manage"},
		forgev1.OperationPlatformServiceCopyRole:                           {"system.role.manage"},
		forgev1.OperationPlatformServiceListPermissions:                    {"system.role.read"},
		forgev1.OperationPlatformServiceListMenus:                          {"system.menu.read"},
		forgev1.OperationPlatformServiceUpdateMenu:                         {"system.menu.manage"},
		forgev1.OperationPlatformServiceListDataFieldPolicies:              {"system.data_policy.read"},
		forgev1.OperationPlatformServiceUpsertDataFieldPolicy:              {"system.data_policy.manage"},
		forgev1.OperationPlatformServiceAuthorizeDataExport:                {"system.data.export"},
		forgev1.OperationPlatformServiceListDataDeletionEvidence:           {"system.data.retention.read"},
		forgev1.OperationPlatformServiceRecordDataDeletionEvidence:         {"system.data.retention.manage"},
		forgev1.OperationPlatformServiceUpdateRolePermissions:              {"system.role.manage"},
		forgev1.OperationPlatformServiceUpdateRoleDataScope:                {"system.role.manage"},
		forgev1.OperationPlatformServiceUpdateUserRoles:                    {"system.user.role.manage"},
		forgev1.OperationPlatformServiceUpdateUserStatus:                   {"system.user.update"},
		forgev1.OperationPlatformServiceUpdateUserEntitlement:              {"system.user.update"},
		forgev1.OperationPlatformServiceUnlockUser:                         {"system.user.update"},
		forgev1.OperationPlatformServiceResetUserPassword:                  {"system.user.update"},
		forgev1.OperationPlatformServiceListSessions:                       {"system.session.read"},
		forgev1.OperationPlatformServiceRevokeSession:                      {"system.session.revoke"},
		forgev1.OperationPlatformServiceListAuditLogs:                      {"system.audit.read"},
		forgev1.OperationPlatformServiceExportAuditLogs:                    {"system.audit.export"},
		forgev1.OperationPlatformServiceVerifyAuditIntegrity:               {"system.audit.verify"},
		forgev1.OperationPlatformServiceListTemporaryRoleGrants:            {"system.temporary_grant.read"},
		forgev1.OperationPlatformServiceCreateTemporaryRoleGrant:           {"system.temporary_grant.manage"},
		forgev1.OperationPlatformServiceRevokeTemporaryRoleGrant:           {"system.temporary_grant.manage"},
		forgev1.OperationPlatformServiceListFederatedIdentityLinks:         {"system.identity_mapping.read"},
		forgev1.OperationPlatformServiceLinkFederatedIdentity:              {"system.identity_mapping.manage"},
		forgev1.OperationPlatformServiceUnlinkFederatedIdentity:            {"system.identity_mapping.manage"},
		forgev1.OperationPlatformServiceListAccessReviews:                  {"system.access_review.read"},
		forgev1.OperationPlatformServiceCreateAccessReview:                 {"system.access_review.manage"},
		forgev1.OperationPlatformServiceListAccessReviewItems:              {"system.access_review.read"},
		forgev1.OperationPlatformServiceDecideAccessReviewItem:             {"system.access_review.manage"},
		forgev1.OperationPlatformServiceListConfigChanges:                  {"system.config.read"},
		forgev1.OperationPlatformServiceCreateConfigChange:                 {"system.config.manage"},
		forgev1.OperationPlatformServiceApproveConfigChange:                {"system.config.manage"},
		forgev1.OperationPlatformServicePublishConfigChange:                {"system.config.manage"},
		forgev1.OperationPlatformServiceRequestConfigRollback:              {"system.config.manage"},
		forgev1.OperationPlatformServiceRollbackConfigChange:               {"system.config.manage"},
		forgev1.OperationApprovalServiceCreateApproval:                     {"approval.request.create"},
		forgev1.OperationApprovalServiceGetApproval:                        {"approval.request.read"},
		forgev1.OperationApprovalServiceListApprovals:                      {"approval.request.read"},
		forgev1.OperationApprovalServiceDecideApproval:                     {"approval.task.decide"},
		forgev1.OperationApprovalServiceTransferApproval:                   {"approval.task.transfer"},
		forgev1.OperationApprovalServiceWithdrawApproval:                   {"approval.request.withdraw"},
	}
}

func PortalRules() map[string][]string {
	return map[string][]string{
		forgev1.OperationPortalServiceAuthorizePortalApplication:   {},
		forgev1.OperationPortalServiceListPortalApplications:       {},
		forgev1.OperationPortalServiceGetPortalApplication:         {},
		forgev1.OperationPortalServiceLaunchPortalApplication:      {},
		forgev1.OperationPortalServiceListPortalFavorites:          {},
		forgev1.OperationPortalServiceAddPortalFavorite:            {},
		forgev1.OperationPortalServiceRemovePortalFavorite:         {},
		forgev1.OperationPortalServiceListRecentPortalApplications: {},
		forgev1.OperationPortalServiceListPortalCategories:         {},
		forgev1.OperationPortalServiceListPortalTags:               {},
		// Identity administrators need the sanitized admin application list to
		// select an application for onboarding; mutation routes remain manage-only.
		forgev1.OperationPortalServiceListAdminPortalApplications:               {"portal.application.manage", "iam.integration.read"},
		forgev1.OperationPortalServiceCreatePortalApplication:                   {"portal.application.manage"},
		forgev1.OperationPortalServiceUpdatePortalApplication:                   {"portal.application.manage"},
		forgev1.OperationPortalServiceDeletePortalApplication:                   {"portal.application.manage"},
		forgev1.OperationPortalServiceCreatePortalCategory:                      {"portal.application.manage"},
		forgev1.OperationPortalServiceUpdatePortalCategory:                      {"portal.application.manage"},
		forgev1.OperationPortalServiceDeletePortalCategory:                      {"portal.application.manage"},
		forgev1.OperationPortalServiceCreatePortalTag:                           {"portal.application.manage"},
		forgev1.OperationPortalServiceUpdatePortalTag:                           {"portal.application.manage"},
		forgev1.OperationPortalServiceDeletePortalTag:                           {"portal.application.manage"},
		forgev1.OperationPortalServiceReplacePortalApplicationPolicies:          {"portal.application.manage"},
		forgev1.OperationPortalServiceListPortalApplicationAccessGrants:         {"portal.application.manage"},
		forgev1.OperationPortalServicePreviewPortalApplicationAccessGrants:      {"portal.application.manage"},
		forgev1.OperationPortalServiceReplacePortalApplicationAccessGrants:      {"portal.application.manage"},
		forgev1.OperationPortalServiceListPortalApplicationEffectiveAccess:      {"portal.application.manage"},
		forgev1.OperationPortalServiceListPortalApplicationRoles:                {"portal.application.manage", "iam.integration.read"},
		forgev1.OperationPortalServiceReplacePortalApplicationRoles:             {"portal.application.manage"},
		forgev1.OperationPortalServiceGetPortalApplicationProvisioningTarget:    {"portal.application.manage", "iam.integration.read"},
		forgev1.OperationPortalServiceUpsertPortalApplicationProvisioningTarget: {"portal.application.manage", "iam.integration.manage"},
		forgev1.OperationPortalServiceRetryPortalApplicationProvisioning:        {"portal.application.manage", "iam.integration.manage"},
		forgev1.OperationPortalServiceGetIdentityOverview:                       {"iam.integration.read"},
		forgev1.OperationPortalServiceGetIdentityConsoleLink:                    {"iam.console.open"},
		forgev1.OperationPortalServiceGetApplicationOnboarding:                  {"iam.integration.read"},
		forgev1.OperationPortalServiceUpsertApplicationIdentityBinding:          {"iam.integration.manage"},
		forgev1.OperationPortalServicePrepareApplicationCredentialApproval:      {"iam.integration.manage"},
		forgev1.OperationPortalServiceVerifyApplicationIdentity:                 {"iam.integration.verify"},
		forgev1.OperationPortalServiceRunApplicationOnboardingChecks:            {"iam.integration.verify"},
		forgev1.OperationPortalServiceSubmitApplicationPublish:                  {"portal.application.publish"},
		forgev1.OperationPortalServicePublishApplication:                        {"portal.application.publish"},
		forgev1.OperationPortalServiceDisableApplication:                        {"portal.application.publish"},
	}
}

func IdentityRules() map[string][]string {
	return map[string][]string{
		forgev1.OperationIdentityServiceListApiTokens:  {"system.api_token.manage"},
		forgev1.OperationIdentityServiceCreateApiToken: {"system.api_token.manage"},
		forgev1.OperationIdentityServiceRevokeApiToken: {"system.api_token.manage"},
	}
}

func Rules() map[string][]string {
	rules := PlatformRules()
	for operation, permissions := range IdentityRules() {
		rules[operation] = permissions
	}
	for operation, permissions := range PortalRules() {
		rules[operation] = permissions
	}
	return rules
}
