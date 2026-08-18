// Velora 领域 API 函数与 react-query 键。
// 字段与后端 JSON 契约一致。

import { apiFetch, buildQuery } from './client'
import type {
  Application,
  AuditLog,
  Category,
  CurrentUser,
  DashboardStats,
  LaunchResult,
  Page,
  RecentItem,
  Tag,
} from '../types'

// --- 认证 ---

export function getMe(): Promise<CurrentUser> {
  return apiFetch<CurrentUser>('/me')
}

export function logout(): Promise<{ status: string }> {
  return apiFetch<{ status: string }>('/auth/logout', { method: 'POST' })
}

/** 跳转 Casdoor 发起 OIDC 登录（整页跳转）。 */
export function oidcLoginUrl(redirect?: string): string {
  const q = redirect ? `?redirect=${encodeURIComponent(redirect)}` : ''
  return `/api/v1/auth/oidc/login${q}`
}

/** 账号密码登录（后端代理 Casdoor OAuth2 password 模式）。成功后返回站内跳转目标。 */
export function loginWithPassword(username: string, password: string, redirect?: string): Promise<{ redirect: string }> {
  return apiFetch<{ redirect: string }>('/auth/login', {
    method: 'POST',
    body: { username, password, redirect: redirect && redirect.startsWith('/') ? redirect : undefined },
  })
}

// --- 应用 ---

export interface ListApplicationsParams {
  keyword?: string
  categoryId?: number
  tagIds?: number[]
  featured?: boolean
  favorites?: boolean
  page?: number
  pageSize?: number
}

export function listApplications(params: ListApplicationsParams = {}): Promise<Page<Application>> {
  return apiFetch(`/applications${buildQuery({
    keyword: params.keyword,
    categoryId: params.categoryId,
    tagId: params.tagIds?.join(','),
    featured: params.featured,
    favorites: params.favorites,
    page: params.page ?? 1,
    pageSize: params.pageSize ?? 24,
  })}`)
}

export function getApplication(id: number | string): Promise<Application> {
  return apiFetch(`/applications/${id}`)
}

export function launchApplication(id: number | string): Promise<LaunchResult> {
  return apiFetch(`/applications/${id}/launch`, { method: 'POST' })
}

export function listRecent(limit = 8): Promise<RecentItem[]> {
  return apiFetch(`/recent${buildQuery({ limit })}`)
}

export function listPopular(limit = 8): Promise<Application[]> {
  return apiFetch(`/popular${buildQuery({ limit })}`)
}

// --- 分类 / 标签 ---

export function listCategories(): Promise<Category[]> {
  return apiFetch('/categories')
}

export function listTags(): Promise<Tag[]> {
  return apiFetch('/tags')
}

// --- 收藏 ---

export function addFavorite(applicationId: number | string): Promise<{ favorited: boolean }> {
  return apiFetch(`/favorites/${applicationId}`, { method: 'POST' })
}

export function removeFavorite(applicationId: number | string): Promise<{ favorited: boolean }> {
  return apiFetch(`/favorites/${applicationId}`, { method: 'DELETE' })
}

// --- 管理端 ---

export interface AdminApplicationInput {
  code: string
  name: string
  description?: string
  keywords?: string
  icon?: string
  categoryId?: number | null
  homeUrl?: string
  launchUrl?: string
  ssoType: string
  casdoorApplicationName?: string
  casdoorClientId?: string
  owner?: string
  department?: string
  status: string
  sort?: number
  isFeatured?: boolean
  healthCheckEnabled?: boolean
  healthCheckUrl?: string
  tagIds?: number[]
  policies?: { policyType: string; value: string }[]
}

export function adminListApplications(params: ListApplicationsParams = {}): Promise<Page<Application>> {
  return apiFetch(`/admin/applications${buildQuery({
    keyword: params.keyword,
    categoryId: params.categoryId,
    page: params.page ?? 1,
    pageSize: params.pageSize ?? 20,
  })}`)
}

export function adminCreateApplication(input: AdminApplicationInput): Promise<Application> {
  return apiFetch('/admin/applications', { method: 'POST', body: input })
}

export function adminUpdateApplication(id: number | string, input: AdminApplicationInput): Promise<Application> {
  return apiFetch(`/admin/applications/${id}`, { method: 'PUT', body: input })
}

export function adminDeleteApplication(id: number | string): Promise<void> {
  return apiFetch(`/admin/applications/${id}`, { method: 'DELETE' })
}

export function adminSetPolicies(id: number | string, policies: { policyType: string; value: string }[]): Promise<unknown> {
  return apiFetch(`/admin/applications/${id}/policies`, { method: 'PUT', body: { policies } })
}

export function adminCreateCategory(input: Partial<Category>): Promise<Category> {
  return apiFetch('/admin/categories', { method: 'POST', body: input })
}

export function adminUpdateCategory(id: number, input: Partial<Category>): Promise<Category> {
  return apiFetch(`/admin/categories/${id}`, { method: 'PUT', body: input })
}

export function adminDeleteCategory(id: number): Promise<void> {
  return apiFetch(`/admin/categories/${id}`, { method: 'DELETE' })
}

export function adminCreateTag(input: Partial<Tag>): Promise<Tag> {
  return apiFetch('/admin/tags', { method: 'POST', body: input })
}

export function adminUpdateTag(id: number, input: Partial<Tag>): Promise<Tag> {
  return apiFetch(`/admin/tags/${id}`, { method: 'PUT', body: input })
}

export function adminDeleteTag(id: number): Promise<void> {
  return apiFetch(`/admin/tags/${id}`, { method: 'DELETE' })
}

export function adminListAuditLogs(params: { page?: number; pageSize?: number; operator?: string; action?: string } = {}): Promise<Page<AuditLog>> {
  return apiFetch(`/admin/audit-logs${buildQuery({
    page: params.page ?? 1,
    pageSize: params.pageSize ?? 20,
    operator: params.operator,
    action: params.action,
  })}`)
}

export function adminDashboard(): Promise<DashboardStats> {
  return apiFetch('/admin/dashboard')
}

export function getPortalSettings(): Promise<{ key: string; value: string }[]> {
  return apiFetch('/portal/settings')
}

export function updatePortalSetting(key: string, value: string): Promise<unknown> {
  return apiFetch('/admin/portal/settings', { method: 'PUT', body: { key, value } })
}

/** 系统版本（公开端点，登录页展示）。 */
export function getSystemVersion(): Promise<{ application: string; version?: string }> {
  return apiFetch('/system/version')
}

// --- react-query 键 ---

export const queryKeys = {
  me: ['me'] as const,
  applications: (params?: unknown) => ['applications', params] as const,
  application: (id: number | string) => ['applications', id] as const,
  recent: ['recent'] as const,
  popular: ['popular'] as const,
  categories: ['categories'] as const,
  tags: ['tags'] as const,
  favorites: ['favorites'] as const,
  adminApplications: (params?: unknown) => ['admin', 'applications', params] as const,
  auditLogs: (params?: unknown) => ['admin', 'audit-logs', params] as const,
  dashboard: ['admin', 'dashboard'] as const,
  portalSettings: ['portal', 'settings'] as const,
}
