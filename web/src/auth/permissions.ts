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

/** Built-in system_admin has an explicit backend superuser boundary. */
export function hasPermission(permissions: string[] | undefined, required: string, roles: string[] = []): boolean {
  if (permissions?.includes(required)) return true
  if (required === AUDIT_READ && permissions?.includes(SYSTEM_AUDIT_READ)) return true
  return permissions?.length === 0 && roles.includes('system_admin')
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
]

export function canAccessAdmin(permissions: string[] | undefined, roles: string[] = []): boolean {
  return hasAnyPermission(permissions, ADMIN_ENTRY_PERMISSIONS, roles)
}
