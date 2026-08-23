package csrf

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	forgev1 "github.com/sevoniva-labs/velora/server/api/gen/go/forge/v1"
	"github.com/sevoniva-labs/velora/server/internal/platform/authn"
)

const csrfCookieName = "velora_csrf"

var protectedOperations = map[string]struct{}{
	forgev1.OperationIdentityServiceLogout:                                  {},
	forgev1.OperationIdentityServiceChangePassword:                          {},
	forgev1.OperationIdentityServiceStepUpAuthentication:                    {},
	forgev1.OperationIdentityServiceCreateApiToken:                          {},
	forgev1.OperationIdentityServiceRevokeApiToken:                          {},
	forgev1.OperationIdentityServiceBeginMFAEnrollment:                      {},
	forgev1.OperationIdentityServiceConfirmMFAEnrollment:                    {},
	forgev1.OperationIdentityServiceDisableMFA:                              {},
	forgev1.OperationPlatformServiceCreateUser:                              {},
	forgev1.OperationPlatformServiceUpdateOrganization:                      {},
	forgev1.OperationPlatformServiceUpdateSecurityPolicy:                    {},
	forgev1.OperationPlatformServiceUpdateRolePermissions:                   {},
	forgev1.OperationPlatformServiceCreateRole:                              {},
	forgev1.OperationPlatformServiceUpdateRole:                              {},
	forgev1.OperationPlatformServiceCopyRole:                                {},
	forgev1.OperationPlatformServiceUpdateRoleDataScope:                     {},
	forgev1.OperationPlatformServiceUpdateUserRoles:                         {},
	forgev1.OperationPlatformServiceUpdateUserStatus:                        {},
	forgev1.OperationPlatformServiceUpdateUserEntitlement:                   {},
	forgev1.OperationPlatformServiceUnlockUser:                              {},
	forgev1.OperationPlatformServiceResetUserPassword:                       {},
	forgev1.OperationPlatformServiceRevokeSession:                           {},
	forgev1.OperationPlatformServiceUpsertDataFieldPolicy:                   {},
	forgev1.OperationPlatformServiceAuthorizeDataExport:                     {},
	forgev1.OperationPlatformServiceRecordDataDeletionEvidence:              {},
	forgev1.OperationPlatformServiceExportAuditLogs:                         {},
	forgev1.OperationPlatformServiceCreateTemporaryRoleGrant:                {},
	forgev1.OperationPlatformServiceRevokeTemporaryRoleGrant:                {},
	forgev1.OperationPlatformServiceLinkFederatedIdentity:                   {},
	forgev1.OperationPlatformServiceUnlinkFederatedIdentity:                 {},
	forgev1.OperationPlatformServiceCreateAccessReview:                      {},
	forgev1.OperationPlatformServiceDecideAccessReviewItem:                  {},
	forgev1.OperationApprovalServiceCreateApproval:                          {},
	forgev1.OperationApprovalServiceDecideApproval:                          {},
	forgev1.OperationApprovalServiceTransferApproval:                        {},
	forgev1.OperationApprovalServiceWithdrawApproval:                        {},
	forgev1.OperationPortalServiceCreatePortalApplication:                   {},
	forgev1.OperationPortalServiceUpdatePortalApplication:                   {},
	forgev1.OperationPortalServiceDeletePortalApplication:                   {},
	forgev1.OperationPortalServiceCreatePortalCategory:                      {},
	forgev1.OperationPortalServiceUpdatePortalCategory:                      {},
	forgev1.OperationPortalServiceDeletePortalCategory:                      {},
	forgev1.OperationPortalServiceCreatePortalTag:                           {},
	forgev1.OperationPortalServiceUpdatePortalTag:                           {},
	forgev1.OperationPortalServiceDeletePortalTag:                           {},
	forgev1.OperationPortalServiceReplacePortalApplicationPolicies:          {},
	forgev1.OperationPortalServiceReplacePortalApplicationRoles:             {},
	forgev1.OperationPortalServiceUpsertPortalApplicationProvisioningTarget: {},
	forgev1.OperationPortalServiceUpsertApplicationIdentityBinding:          {},
	forgev1.OperationPortalServicePrepareApplicationCredentialApproval:      {},
	forgev1.OperationPortalServiceVerifyApplicationIdentity:                 {},
	forgev1.OperationPortalServiceRunApplicationOnboardingChecks:            {},
	forgev1.OperationPortalServiceSubmitApplicationPublish:                  {},
	forgev1.OperationPortalServicePublishApplication:                        {},
	forgev1.OperationPortalServiceDisableApplication:                        {},
}

func Server() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, kratoserrors.Forbidden("CSRF_MISMATCH", "transport context is required")
			}
			if _, protected := protectedOperations[tr.Operation()]; !protected {
				return next(ctx, req)
			}
			principal, authenticated := authn.Principal(ctx)
			if !authenticated {
				return nil, kratoserrors.Unauthorized("UNAUTHENTICATED", "authentication is required")
			}
			if strings.EqualFold(principal.Type, "TOKEN") {
				return next(ctx, req)
			}
			headerToken := strings.TrimSpace(tr.RequestHeader().Get("X-CSRF-Token"))
			cookieToken := cookieValue(tr.RequestHeader().Get("Cookie"), csrfCookieName)
			if headerToken == "" || cookieToken == "" || subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) != 1 {
				return nil, kratoserrors.Forbidden("CSRF_MISMATCH", "CSRF validation failed")
			}
			return next(ctx, req)
		}
	}
}

func cookieValue(raw, name string) string {
	request := &http.Request{Header: http.Header{"Cookie": []string{raw}}}
	cookie, err := request.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
