import { apiFetch } from './client'
import type { AdminSession, AdminUser, ApplicationAccessGrant, ApplicationAccessImpact, ApplicationEffectiveAccess, Department, PlatformPermission, PlatformRole, Position, UserAssignment, UserGroup } from '../types'

export type DepartmentInput = Pick<Department, 'name' | 'parentId' | 'status' | 'sortOrder'> & { departmentKey?: string }
export type PositionInput = Pick<Position, 'name' | 'description' | 'departmentId' | 'status' | 'sortOrder'> & { positionKey?: string }
export type UserGroupInput = Pick<UserGroup, 'name' | 'description' | 'status'> & { groupKey?: string }

export async function listDepartments(): Promise<Department[]> {
  return (await apiFetch<{ departments?: Department[] }>('/admin/departments')).departments ?? []
}

export async function createDepartment(input: DepartmentInput): Promise<Department> {
  const data = await apiFetch<{ department: Department }>('/admin/departments', { method: 'POST', body: input })
  return data.department
}

export async function updateDepartment(id: string, input: DepartmentInput): Promise<Department> {
  const data = await apiFetch<{ department: Department }>(`/admin/departments/${encodeURIComponent(id)}`, { method: 'PATCH', body: input })
  return data.department
}

export async function listPositions(): Promise<Position[]> {
  return (await apiFetch<{ positions?: Position[] }>('/admin/positions')).positions ?? []
}

export async function createPosition(input: PositionInput): Promise<Position> {
  const data = await apiFetch<{ position: Position }>('/admin/positions', { method: 'POST', body: input })
  return data.position
}

export async function updatePosition(id: string, input: PositionInput): Promise<Position> {
  const data = await apiFetch<{ position: Position }>(`/admin/positions/${encodeURIComponent(id)}`, { method: 'PATCH', body: input })
  return data.position
}

export async function listUserGroups(): Promise<UserGroup[]> {
  return (await apiFetch<{ userGroups?: UserGroup[] }>('/admin/user-groups')).userGroups ?? []
}

export async function createUserGroup(input: UserGroupInput): Promise<UserGroup> {
  const data = await apiFetch<{ userGroup: UserGroup }>('/admin/user-groups', { method: 'POST', body: input })
  return data.userGroup
}

export async function updateUserGroup(id: string, input: UserGroupInput): Promise<UserGroup> {
  const data = await apiFetch<{ userGroup: UserGroup }>(`/admin/user-groups/${encodeURIComponent(id)}`, { method: 'PATCH', body: input })
  return data.userGroup
}

export function replaceUserGroupMembers(id: string, userIds: string[]): Promise<unknown> {
  return apiFetch(`/admin/user-groups/${encodeURIComponent(id)}/members`, { method: 'PUT', body: { userIds } })
}

export function replaceUserGroupRoles(id: string, roles: string[]): Promise<unknown> {
  return apiFetch(`/admin/user-groups/${encodeURIComponent(id)}/roles`, { method: 'PUT', body: { roles } })
}

export async function listPlatformRoles(): Promise<PlatformRole[]> {
  return (await apiFetch<{ roles?: PlatformRole[] }>('/admin/roles')).roles ?? []
}

export async function listPlatformPermissions(): Promise<PlatformPermission[]> {
  return (await apiFetch<{ permissions?: PlatformPermission[] }>('/admin/permissions')).permissions ?? []
}

export async function updateRolePermissions(roleKey: string, permissions: string[]): Promise<PlatformRole> {
  const data = await apiFetch<{ role: PlatformRole }>(`/admin/roles/${encodeURIComponent(roleKey)}/permissions`, { method: 'PUT', body: { permissions } })
  return data.role
}

export async function updateRoleDataScope(roleKey: string, dataScope: string, departmentIds: string[]): Promise<PlatformRole> {
  const data = await apiFetch<{ role: PlatformRole }>(`/admin/roles/${encodeURIComponent(roleKey)}/data-scope`, { method: 'PUT', body: { dataScope, departmentIds } })
  return data.role
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

export async function updateUserRoles(userId: string, roles: string[]): Promise<AdminUser> {
  const data = await apiFetch<{ user: AdminUser }>(`/admin/users/${encodeURIComponent(userId)}/roles`, { method: 'PATCH', body: { roles } })
  return data.user
}

export async function listApplicationAccessGrants(applicationId: string): Promise<ApplicationAccessGrant[]> {
  return (await apiFetch<{ grants?: ApplicationAccessGrant[] }>(`/admin/portal/applications/${encodeURIComponent(applicationId)}/access-grants`)).grants ?? []
}

export async function previewApplicationAccessGrants(applicationId: string, grants: ApplicationAccessGrant[]): Promise<{ impact: ApplicationAccessImpact; effectiveAccess: ApplicationEffectiveAccess[] }> {
  return apiFetch(`/admin/portal/applications/${encodeURIComponent(applicationId)}/access-grants:preview`, { method: 'POST', body: { grants } })
}

export async function replaceApplicationAccessGrants(applicationId: string, grants: ApplicationAccessGrant[], approvalId?: string): Promise<{ grants: ApplicationAccessGrant[]; impact: ApplicationAccessImpact }> {
  return apiFetch(`/admin/portal/applications/${encodeURIComponent(applicationId)}/access-grants`, { method: 'PUT', body: { grants, approvalId: approvalId ?? '' } })
}

export async function listApplicationEffectiveAccess(applicationId: string): Promise<ApplicationEffectiveAccess[]> {
  return (await apiFetch<{ effectiveAccess?: ApplicationEffectiveAccess[] }>(`/admin/portal/applications/${encodeURIComponent(applicationId)}/effective-access`)).effectiveAccess ?? []
}
