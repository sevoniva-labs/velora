import { apiFetch } from './client'
import type { AccessReview, AccessReviewItem, AdminSession, AdminUser, ApplicationAccessGrant, ApplicationAccessImpact, ApplicationEffectiveAccess, ApprovalRequest, ConfigChange, Department, OrganizationInfo, PlatformPermission, PlatformRole, Position, SecurityPolicy, TemporaryRoleGrant, UserAssignment, UserEffectiveApplicationAccess, UserGroup } from '../types'

export type DepartmentInput = Pick<Department, 'name' | 'parentId' | 'status' | 'sortOrder'> & { departmentKey?: string }
export type PositionInput = Pick<Position, 'name' | 'description' | 'departmentId' | 'status' | 'sortOrder'> & { positionKey?: string }
export type UserGroupInput = Pick<UserGroup, 'name' | 'description' | 'status'> & { groupKey?: string }
export type OrganizationInput = Pick<OrganizationInfo, 'name' | 'description' | 'status' | 'maxUsers' | 'maxActiveSessions'>

export function messageFromResponse<T>(value: T | Record<string, T>, field: string): T {
  if (value && typeof value === 'object' && field in value) return (value as Record<string, T>)[field]
  return value as T
}

function normalizeUserGroup(item: UserGroup): UserGroup {
  return { ...item, roles: item.roles ?? [], memberIds: item.memberIds ?? [], memberCount: item.memberCount ?? item.memberIds?.length ?? 0 }
}

function normalizePlatformRole(item: PlatformRole): PlatformRole {
  return { ...item, permissions: item.permissions ?? [], dataScopeDepartmentIds: item.dataScopeDepartmentIds ?? [] }
}

function normalizeAccessGrant(item: ApplicationAccessGrant): ApplicationAccessGrant {
  return { ...item, roles: item.roles ?? [] }
}

function normalizeEffectiveAccess(item: ApplicationEffectiveAccess): ApplicationEffectiveAccess {
  return { ...item, roles: item.roles ?? [], sourceGrantIds: item.sourceGrantIds ?? [] }
}

export async function listDepartments(): Promise<Department[]> {
  return (await apiFetch<{ departments?: Department[] }>('/admin/departments')).departments ?? []
}

export async function getOrganization(): Promise<OrganizationInfo> {
  return messageFromResponse(await apiFetch<{ organization: OrganizationInfo }>('/admin/organization'), 'organization')
}

export async function updateOrganization(input: OrganizationInput): Promise<OrganizationInfo> {
  return messageFromResponse(await apiFetch<{ organization: OrganizationInfo }>('/admin/organization', { method: 'PATCH', body: input }), 'organization')
}

export async function createDepartment(input: DepartmentInput): Promise<Department> {
  const data = await apiFetch<{ department: Department }>('/admin/departments', { method: 'POST', body: input })
  return messageFromResponse(data, 'department')
}

export async function updateDepartment(id: string, input: DepartmentInput): Promise<Department> {
  const data = await apiFetch<{ department: Department }>(`/admin/departments/${encodeURIComponent(id)}`, { method: 'PATCH', body: input })
  return messageFromResponse(data, 'department')
}

export async function listPositions(): Promise<Position[]> {
  return (await apiFetch<{ positions?: Position[] }>('/admin/positions')).positions ?? []
}

export async function createPosition(input: PositionInput): Promise<Position> {
  const data = await apiFetch<{ position: Position }>('/admin/positions', { method: 'POST', body: input })
  return messageFromResponse(data, 'position')
}

export async function updatePosition(id: string, input: PositionInput): Promise<Position> {
  const data = await apiFetch<{ position: Position }>(`/admin/positions/${encodeURIComponent(id)}`, { method: 'PATCH', body: input })
  return messageFromResponse(data, 'position')
}

export async function listUserGroups(): Promise<UserGroup[]> {
  return ((await apiFetch<{ userGroups?: UserGroup[] }>('/admin/user-groups')).userGroups ?? []).map(normalizeUserGroup)
}

export async function createUserGroup(input: UserGroupInput): Promise<UserGroup> {
  const data = await apiFetch<{ userGroup: UserGroup }>('/admin/user-groups', { method: 'POST', body: input })
  return normalizeUserGroup(messageFromResponse(data, 'userGroup'))
}

export async function updateUserGroup(id: string, input: UserGroupInput): Promise<UserGroup> {
  const data = await apiFetch<{ userGroup: UserGroup }>(`/admin/user-groups/${encodeURIComponent(id)}`, { method: 'PATCH', body: input })
  return normalizeUserGroup(messageFromResponse(data, 'userGroup'))
}

export function replaceUserGroupMembers(id: string, userIds: string[]): Promise<unknown> {
  return apiFetch(`/admin/user-groups/${encodeURIComponent(id)}/members`, { method: 'PUT', body: { userIds } })
}

export function replaceUserGroupRoles(id: string, roles: string[]): Promise<unknown> {
  return apiFetch(`/admin/user-groups/${encodeURIComponent(id)}/roles`, { method: 'PUT', body: { roles } })
}

export async function listPlatformRoles(): Promise<PlatformRole[]> {
  return ((await apiFetch<{ roles?: PlatformRole[] }>('/admin/roles')).roles ?? []).map(normalizePlatformRole)
}

export async function createPlatformRole(input: { roleKey: string; name: string; description?: string }): Promise<PlatformRole> {
  return normalizePlatformRole(messageFromResponse(await apiFetch<{ role: PlatformRole }>('/admin/roles', { method: 'POST', body: input }), 'role'))
}

export async function updatePlatformRole(roleKey: string, input: { name: string; description?: string; status: 'ACTIVE' | 'DISABLED' }): Promise<PlatformRole> {
  return normalizePlatformRole(messageFromResponse(await apiFetch<{ role: PlatformRole }>(`/admin/roles/${encodeURIComponent(roleKey)}`, { method: 'PATCH', body: input }), 'role'))
}

export async function copyPlatformRole(sourceRoleKey: string, input: { roleKey: string; name: string; description?: string }): Promise<PlatformRole> {
  return normalizePlatformRole(messageFromResponse(await apiFetch<{ role: PlatformRole }>(`/admin/roles/${encodeURIComponent(sourceRoleKey)}:copy`, { method: 'POST', body: input }), 'role'))
}

export async function listPlatformPermissions(): Promise<PlatformPermission[]> {
  return (await apiFetch<{ permissions?: PlatformPermission[] }>('/admin/permissions')).permissions ?? []
}

export async function getSecurityPolicy(): Promise<SecurityPolicy> {
  return messageFromResponse(await apiFetch<{ policy: SecurityPolicy }>('/admin/security-config'), 'policy')
}

export async function updateSecurityPolicy(policy: SecurityPolicy, approvalId: string): Promise<SecurityPolicy> {
  return messageFromResponse(await apiFetch<{ policy: SecurityPolicy }>('/admin/security-config', { method: 'PUT', body: { policy, approvalId } }), 'policy')
}

export async function updateRolePermissions(roleKey: string, permissions: string[], approvalId?: string): Promise<PlatformRole> {
  const data = await apiFetch<{ role: PlatformRole }>(`/admin/roles/${encodeURIComponent(roleKey)}/permissions`, { method: 'PUT', body: { permissions, approvalId: approvalId ?? '' } })
  return normalizePlatformRole(messageFromResponse(data, 'role'))
}

export async function updateRoleDataScope(roleKey: string, dataScope: string, departmentIds: string[], approvalId?: string): Promise<PlatformRole> {
  const data = await apiFetch<{ role: PlatformRole }>(`/admin/roles/${encodeURIComponent(roleKey)}/data-scope`, { method: 'PUT', body: { dataScope, departmentIds, approvalId: approvalId ?? '' } })
  return normalizePlatformRole(messageFromResponse(data, 'role'))
}

export async function listSessions(): Promise<AdminSession[]> {
  return (await apiFetch<{ sessions?: AdminSession[] }>('/admin/sessions?limit=100')).sessions ?? []
}

export function revokeSession(id: string): Promise<unknown> {
  return apiFetch(`/admin/sessions/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export async function listUserAssignments(userId: string): Promise<UserAssignment[]> {
  return (await apiFetch<{ assignments?: UserAssignment[] }>(`/admin/users/${encodeURIComponent(userId)}/assignments`)).assignments ?? []
}

export function replaceUserAssignments(userId: string, assignments: UserAssignment[]): Promise<unknown> {
  return apiFetch(`/admin/users/${encodeURIComponent(userId)}/assignments`, { method: 'PUT', body: { assignments } })
}

export async function listUserEffectiveApplicationAccess(userId: string): Promise<UserEffectiveApplicationAccess[]> {
  return (await apiFetch<{ accesses?: UserEffectiveApplicationAccess[] }>(`/admin/users/${encodeURIComponent(userId)}/effective-application-access`)).accesses ?? []
}

export async function updateUserRoles(userId: string, roles: string[], approvalId?: string): Promise<AdminUser> {
  const data = await apiFetch<{ user: AdminUser }>(`/admin/users/${encodeURIComponent(userId)}/roles`, { method: 'PATCH', body: { roles, approvalId: approvalId ?? '' } })
  return messageFromResponse(data, 'user')
}

export function unlockUser(userId: string): Promise<unknown> {
  return apiFetch(`/admin/users/${encodeURIComponent(userId)}/unlock`, { method: 'POST', body: {} })
}

export function resetUserPassword(userId: string, password: string, approvalId: string): Promise<unknown> {
  return apiFetch(`/admin/users/${encodeURIComponent(userId)}/reset-password`, { method: 'POST', body: { password, approvalId } })
}

export async function verifyAuditIntegrity(): Promise<boolean> {
  return Boolean((await apiFetch<{ verified?: boolean }>('/admin/audit-logs/integrity')).verified)
}

export function getSystemReadiness(): Promise<{ status: string; dependencies: { name: string; status: string }[] }> {
  return apiFetch('/system/readiness')
}

export async function exportAuditLogs(format: 'json' | 'csv', limit: number, approvalId: string): Promise<{ content: string; contentType: string; filename: string }> {
  return apiFetch(`/admin/audit-logs/export?format=${encodeURIComponent(format)}&limit=${limit}&approval_id=${encodeURIComponent(approvalId)}`)
}

export async function listApplicationAccessGrants(applicationId: string): Promise<ApplicationAccessGrant[]> {
  return ((await apiFetch<{ grants?: ApplicationAccessGrant[] }>(`/admin/portal/applications/${encodeURIComponent(applicationId)}/access-grants`)).grants ?? []).map(normalizeAccessGrant)
}

export async function previewApplicationAccessGrants(applicationId: string, grants: ApplicationAccessGrant[]): Promise<{ impact: ApplicationAccessImpact; effectiveAccess: ApplicationEffectiveAccess[] }> {
  return apiFetch(`/admin/portal/applications/${encodeURIComponent(applicationId)}/access-grants:preview`, { method: 'POST', body: { grants } })
}

export async function replaceApplicationAccessGrants(applicationId: string, grants: ApplicationAccessGrant[], approvalId?: string): Promise<{ grants: ApplicationAccessGrant[]; impact: ApplicationAccessImpact }> {
  return apiFetch(`/admin/portal/applications/${encodeURIComponent(applicationId)}/access-grants`, { method: 'PUT', body: { grants, approvalId: approvalId ?? '' } })
}

export async function listApplicationEffectiveAccess(applicationId: string): Promise<ApplicationEffectiveAccess[]> {
  return ((await apiFetch<{ effectiveAccess?: ApplicationEffectiveAccess[] }>(`/admin/portal/applications/${encodeURIComponent(applicationId)}/effective-access`)).effectiveAccess ?? []).map(normalizeEffectiveAccess)
}

export async function listApprovals(): Promise<ApprovalRequest[]> {
  return (await apiFetch<{ approvals?: ApprovalRequest[] }>('/approvals')).approvals ?? []
}

export async function createApproval(input: { requestType: string; action: string; resource: string; resourceId: string; summary: string; payloadJson: string; approverIds: string[]; expiresInSeconds?: number }): Promise<ApprovalRequest> {
  return messageFromResponse(await apiFetch<{ approval: ApprovalRequest }>('/approvals', { method: 'POST', body: { ...input, mode: 'ANY', requiredApprovals: 1, expiresInSeconds: input.expiresInSeconds ?? 86_400 } }), 'approval')
}

export async function decideApproval(id: string, decision: 'APPROVE' | 'REJECT', comment: string): Promise<ApprovalRequest> {
  return messageFromResponse(await apiFetch<{ approval: ApprovalRequest }>(`/approvals/${encodeURIComponent(id)}/decisions`, { method: 'POST', body: { decision, comment } }), 'approval')
}

export async function listTemporaryRoleGrants(): Promise<TemporaryRoleGrant[]> {
  return (await apiFetch<{ grants?: TemporaryRoleGrant[] }>('/admin/temporary-role-grants')).grants ?? []
}

export async function createTemporaryRoleGrant(input: { userId: string; roleKey: string; reason: string; validFrom: string; validUntil: string; approvalId: string }): Promise<TemporaryRoleGrant> {
  return messageFromResponse(await apiFetch<{ grant: TemporaryRoleGrant }>('/admin/temporary-role-grants', { method: 'POST', body: input }), 'grant')
}

export function revokeTemporaryRoleGrant(id: string, reason: string): Promise<unknown> {
  return apiFetch(`/admin/temporary-role-grants/${encodeURIComponent(id)}:revoke`, { method: 'POST', body: { reason } })
}

export async function listAccessReviews(): Promise<AccessReview[]> {
  return (await apiFetch<{ reviews?: AccessReview[] }>('/admin/access-reviews')).reviews ?? []
}

export async function createAccessReview(reviewerId: string, dueAt: string): Promise<AccessReview> {
  return messageFromResponse(await apiFetch<{ review: AccessReview }>('/admin/access-reviews', { method: 'POST', body: { reviewerId, dueAt } }), 'review')
}

export async function listAccessReviewItems(reviewId: string): Promise<AccessReviewItem[]> {
  return (await apiFetch<{ items?: AccessReviewItem[] }>(`/admin/access-reviews/${encodeURIComponent(reviewId)}/items`)).items ?? []
}

export function decideAccessReviewItem(reviewId: string, itemId: string, decision: 'APPROVE' | 'REVOKE', reason: string): Promise<unknown> {
  return apiFetch(`/admin/access-reviews/${encodeURIComponent(reviewId)}/items/${encodeURIComponent(itemId)}/decisions`, { method: 'POST', body: { decision, reason } })
}

export async function listConfigChanges(): Promise<ConfigChange[]> {
  return (await apiFetch<{ changes?: ConfigChange[] }>('/admin/config-changes')).changes ?? []
}

export async function createConfigChange(input: Omit<ConfigChange, 'id' | 'createdBy' | 'approvedBy' | 'approvalId' | 'state' | 'updatedAt'>): Promise<ConfigChange> {
  return messageFromResponse(await apiFetch<{ change: ConfigChange }>('/admin/config-changes', { method: 'POST', body: input }), 'change')
}

export async function transitionConfigChange(id: string, action: 'APPROVE' | 'PUBLISH' | 'REQUEST_ROLLBACK' | 'ROLLBACK', approvalId: string): Promise<ConfigChange> {
  const path = action === 'APPROVE' ? 'approve' : action === 'PUBLISH' ? 'publish' : action === 'REQUEST_ROLLBACK' ? 'rollback-request' : 'rollback'
  return messageFromResponse(await apiFetch<{ change: ConfigChange }>(`/admin/config-changes/${encodeURIComponent(id)}/${path}`, { method: 'POST', body: { approvalId } }), 'change')
}
