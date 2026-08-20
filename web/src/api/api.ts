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
  TodoListResult,
  MailAccount,
  MailCapabilities,
  MailMessage,
  MailProviderProfile,
  TodoItem,
  TodoKind,
  TodoPriority,
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

/** 本地开发密码登录；生产环境由后端固定切换为 Casdoor OIDC。 */
export async function loginWithPassword(
  username: string,
  password: string,
  redirect?: string,
  _turnstileToken?: string,
): Promise<{ redirect: string }> {
  await apiFetch('/auth/login', {
    method: 'POST',
    body: {
      // go-antd-fullstack 的 Proto/HTTP 契约使用 loginName/organization。
      // 保留此适配在 API 层，不改变登录页 UI；redirect/Turnstile 由旧前端
      // 传入但当前后端契约不消费，登录成功后由页面回到根路由。
      loginName: username,
      organization: 'default',
      password,
    },
  })
  return { redirect: redirect && redirect.startsWith('/') ? redirect : '/' }
}

/** 登录页人机验证配置（Cloudflare Turnstile；未启用时 enabled=false）。 */
export function getTurnstileConfig(): Promise<{ enabled: boolean; siteKey: string }> {
  return apiFetch('/auth/turnstile-config')
}

// --- 用户中心（Phase C4 自助） ---

export interface UserProfile extends CurrentUser {
  admin: boolean
}

export interface SessionDevice {
  sessionId: string
  userAgent: string
  ip: string
  lastActiveAt: string
  expiresAt: string
  revokedAt?: string
  current: boolean
}

export function getUserProfile(): Promise<UserProfile> {
  return apiFetch<UserProfile>('/user-center/profile')
}

export function changePassword(oldPassword: string, newPassword: string): Promise<{ status: string; message: string }> {
  return apiFetch('/user-center/change-password', {
    method: 'POST',
    body: { oldPassword, newPassword },
  })
}

export function listSessions(): Promise<SessionDevice[]> {
  return apiFetch<SessionDevice[]>('/auth/sessions')
}

export function revokeSession(sessionId: string): Promise<{ revoked: string }> {
  return apiFetch(`/auth/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' })
}

export function revokeAllSessions(): Promise<{ status: string }> {
  return apiFetch('/auth/sessions', { method: 'DELETE' })
}

// --- OIDC Provider 客户端管理（Phase B6 管理后台） ---

export interface OIDCClient {
  clientId: string
  name: string
  redirectUris: string[]
  grantTypes: string[]
  scopes: string[]
  createdAt: string
}

export function listOIDCClients(applicationId: number): Promise<OIDCClient[]> {
  return apiFetch(`/admin/applications/${applicationId}/oidc-clients`)
}

export function createOIDCClient(
  applicationId: number,
  redirectUris: string[],
): Promise<{ client: OIDCClient; clientSecret: string }> {
  return apiFetch(`/admin/applications/${applicationId}/oidc-clients`, {
    method: 'POST',
    body: { redirectUris },
  })
}

export function revokeOIDCClient(clientId: string): Promise<{ status: string }> {
  return apiFetch(`/admin/applications/oidc-clients/${encodeURIComponent(clientId)}`, { method: 'DELETE' })
}

// --- 集成令牌（Service Account，Phase D3） ---

export interface IntegrationToken {
  id: number
  name: string
  scopes: string[]
  createdBy: string
  expiresAt: string | null
  lastUsedAt: string | null
  revoked: boolean
}

export function listIntegrationTokens(): Promise<IntegrationToken[]> {
  return apiFetch('/admin/integration-tokens')
}

export function createIntegrationToken(input: {
  name: string
  scopes: string[]
  expiresInDays?: number
}): Promise<{ token: string; message: string }> {
  const body: Record<string, unknown> = { name: input.name, scopes: input.scopes }
  if (input.expiresInDays) {
    // 服务端按天计算过期时间（避免客户端时区差异）
    body.expiresAt = new Date(Date.now() + input.expiresInDays * 24 * 3600 * 1000).toISOString()
  }
  return apiFetch('/admin/integration-tokens', { method: 'POST', body })
}

export function revokeIntegrationToken(id: number): Promise<{ status: string }> {
  return apiFetch(`/admin/integration-tokens/${id}`, { method: 'DELETE' })
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
  return apiFetch(`/applications/recent${buildQuery({ limit })}`)
}

export function listPopular(limit = 8): Promise<Application[]> {
  return apiFetch(`/applications/popular${buildQuery({ limit })}`)
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

// --- 待办中心 ---

export function listTodos(params: { status?: 'open' | 'done' | 'all'; limit?: number } = {}): Promise<TodoListResult> {
  return apiFetch(`/todos${buildQuery({ status: params.status ?? 'open', limit: params.limit ?? 50 })}`)
}

export function markTodoDone(id: number): Promise<{ done: boolean }> {
  return apiFetch(`/todos/${id}/done`, { method: 'PATCH' })
}

// --- 邮件（企业邮箱） ---

export interface BindMailAccountInput {
  provider: string
  email: string
  password: string
  displayName?: string
  imapHost?: string
  imapPort?: number
  smtpHost?: string
  smtpPort?: number
}

export interface ListMailMessagesParams {
  accountId?: number
  unread?: boolean
  starred?: boolean
  keyword?: string
  page?: number
  pageSize?: number
}

export function listMailProviders(): Promise<{ profiles: MailProviderProfile[]; capabilities: MailCapabilities }> {
  return apiFetch('/mail/providers')
}

export function listMailAccounts(): Promise<MailAccount[]> {
  return apiFetch('/mail/accounts')
}

export function bindMailAccount(input: BindMailAccountInput): Promise<MailAccount> {
  return apiFetch('/mail/accounts', { method: 'POST', body: input })
}

export function unbindMailAccount(id: number): Promise<{ deleted: boolean }> {
  return apiFetch(`/mail/accounts/${id}`, { method: 'DELETE' })
}

export function testMailAccount(id: number): Promise<{ ok: boolean; error?: string }> {
  return apiFetch(`/mail/accounts/${id}/test`, { method: 'POST' })
}

export function syncMailAccount(id: number): Promise<MailAccount> {
  return apiFetch(`/mail/accounts/${id}/sync`, { method: 'POST' })
}

export function listMailMessages(params: ListMailMessagesParams = {}): Promise<Page<MailMessage>> {
  return apiFetch(`/mail/messages${buildQuery({
    accountId: params.accountId,
    unread: params.unread,
    starred: params.starred,
    keyword: params.keyword,
    page: params.page ?? 1,
    pageSize: params.pageSize ?? 20,
  })}`)
}

export function getMailMessage(id: number): Promise<{ message: MailMessage; bodyError?: string }> {
  return apiFetch(`/mail/messages/${id}`)
}

export function setMailRead(id: number, read: boolean): Promise<{ read: boolean }> {
  return apiFetch(`/mail/messages/${id}/read`, { method: 'POST', body: { read } })
}

export function setMailStar(id: number, starred: boolean): Promise<{ starred: boolean }> {
  return apiFetch(`/mail/messages/${id}/star`, { method: 'POST', body: { starred } })
}

export interface ConvertMailToTodoInput {
  title?: string
  priority?: TodoPriority
  kind?: TodoKind
  dueAt?: string | null
}

export function convertMailToTodo(id: number, input: ConvertMailToTodoInput): Promise<TodoItem> {
  return apiFetch(`/mail/messages/${id}/todo`, { method: 'POST', body: input })
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

export function adminSyncApplications(): Promise<{ total: number; created: number; updated: number }> {
  return apiFetch('/admin/applications/sync', { method: 'POST' })
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
  todos: ['todos'] as const,
  mailAccounts: ['mail', 'accounts'] as const,
  mailProviders: ['mail', 'providers'] as const,
  mailMessages: (params?: unknown) => ['mail', 'messages', params] as const,
  mailMessage: (id: number) => ['mail', 'messages', 'detail', id] as const,
  adminApplications: (params?: unknown) => ['admin', 'applications', params] as const,
  auditLogs: (params?: unknown) => ['admin', 'audit-logs', params] as const,
  dashboard: ['admin', 'dashboard'] as const,
  portalSettings: ['portal', 'settings'] as const,
}
