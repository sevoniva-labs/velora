/**
 * Frontend permission helpers. The backend remains the source of truth; these
 * helpers only decide which navigation and controls should be rendered.
 */

export const PORTAL_READ = 'portal.application.read'
export const PORTAL_MANAGE = 'portal.application.manage'
export const PORTAL_PUBLISH = 'portal.application.publish'
export const IDENTITY_READ = 'iam.integration.read'
export const IDENTITY_MANAGE = 'iam.integration.manage'
export const IDENTITY_VERIFY = 'iam.integration.verify'
export const IDENTITY_CONSOLE = 'iam.console.open'
export const AUDIT_READ = 'audit.read'
export const SYSTEM_AUDIT_READ = 'system.audit.read'
export const API_TOKEN_MANAGE = 'system.api_token.manage'
export const SYSTEM_USER_READ = 'system.user.read'
export const SYSTEM_USER_CREATE = 'system.user.create'
export const SYSTEM_USER_UPDATE = 'system.user.update'
export const SYSTEM_USER_ROLE_MANAGE = 'system.user.role.manage'
export const SYSTEM_USER_ASSIGNMENT_READ = 'system.user.assignment.read'
export const SYSTEM_USER_ASSIGNMENT_MANAGE = 'system.user.assignment.manage'
export const SYSTEM_DEPARTMENT_READ = 'system.department.read'
export const SYSTEM_DEPARTMENT_MANAGE = 'system.department.manage'
export const SYSTEM_POSITION_READ = 'system.position.read'
export const SYSTEM_POSITION_MANAGE = 'system.position.manage'
export const SYSTEM_USER_GROUP_READ = 'system.user_group.read'
export const SYSTEM_USER_GROUP_MANAGE = 'system.user_group.manage'
export const SYSTEM_ROLE_READ = 'system.role.read'
export const SYSTEM_ROLE_MANAGE = 'system.role.manage'
export const SYSTEM_SESSION_READ = 'system.session.read'
export const SYSTEM_SESSION_REVOKE = 'system.session.revoke'
export const SYSTEM_TEMPORARY_GRANT_READ = 'system.temporary_grant.read'
export const SYSTEM_TEMPORARY_GRANT_MANAGE = 'system.temporary_grant.manage'
export const SYSTEM_ACCESS_REVIEW_READ = 'system.access_review.read'
export const SYSTEM_ACCESS_REVIEW_MANAGE = 'system.access_review.manage'
export const APPROVAL_REQUEST_READ = 'approval.request.read'
export const APPROVAL_REQUEST_CREATE = 'approval.request.create'
export const APPROVAL_TASK_DECIDE = 'approval.task.decide'
export const SYSTEM_CONFIG_READ = 'system.config.read'
export const SYSTEM_CONFIG_MANAGE = 'system.config.manage'
export const SYSTEM_SECURITY_MANAGE = 'system.security.manage'

/** Built-in system_admin has an explicit backend superuser boundary. */
export function hasPermission(permissions: string[] | undefined, required: string, roles: string[] = []): boolean {
  if (permissions?.includes(required)) return true
  if (required === AUDIT_READ && permissions?.includes(SYSTEM_AUDIT_READ)) return true
  return roles.includes('system_admin')
}

export function hasAnyPermission(
  permissions: string[] | undefined,
  required: string[],
  roles: string[] = [],
): boolean {
  return required.some((permission) => hasPermission(permissions, permission, roles))
}

export const ADMIN_ENTRY_PERMISSIONS = [
  PORTAL_MANAGE,
  PORTAL_PUBLISH,
  IDENTITY_READ,
  IDENTITY_MANAGE,
  IDENTITY_VERIFY,
  IDENTITY_CONSOLE,
  AUDIT_READ,
  API_TOKEN_MANAGE,
  SYSTEM_USER_READ,
  SYSTEM_DEPARTMENT_READ,
  SYSTEM_POSITION_READ,
  SYSTEM_USER_GROUP_READ,
  SYSTEM_ROLE_READ,
  SYSTEM_SESSION_READ,
  SYSTEM_TEMPORARY_GRANT_READ,
  SYSTEM_ACCESS_REVIEW_READ,
  APPROVAL_REQUEST_READ,
  APPROVAL_TASK_DECIDE,
  SYSTEM_CONFIG_READ,
]

export function canAccessAdmin(permissions: string[] | undefined, roles: string[] = []): boolean {
  return hasAnyPermission(permissions, ADMIN_ENTRY_PERMISSIONS, roles)
}
