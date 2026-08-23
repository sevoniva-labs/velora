// Velora Web API 适配层。
// 后端基座使用 Kratos + protobuf HTTP，路由统一挂在 /api/v1，响应字段使用
// proto 的 snake_case。页面继续使用历史领域模型，因此所有路径、请求字段和
// 响应模型的兼容逻辑集中在本文件，页面不再拼接后端 URL。

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
  AccessPolicy,
  IdentityBinding,
  ApplicationVerification,
  AdminUser,
  ApplicationEntitlement,
} from '../types'
import { canAccessAdmin } from '../auth/permissions'

type AnyRecord = Record<string, any>

function record(value: unknown): AnyRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as AnyRecord) : {}
}

function listFrom(value: unknown, ...keys: string[]): AnyRecord[] {
  if (Array.isArray(value)) return value as AnyRecord[]
  const object = record(value)
  for (const key of keys) if (Array.isArray(object[key])) return object[key] as AnyRecord[]
  return []
}

function asId(value: unknown): string {
  return value == null ? '' : String(value)
}

function mapUser(value: unknown): CurrentUser {
  const user = record(value)
  const roles = Array.isArray(user.roles) ? user.roles.map(String) : []
  const permissions = Array.isArray(user.permissions) ? user.permissions.map(String) : []
  // Keep the legacy field for old screens, but derive it from the same
  // permission set used by navigation and route guards.
  const admin = canAccessAdmin(permissions, roles)
  return {
    id: asId(user.id),
    username: String(user.loginName ?? user.username ?? ''),
    displayName: String(user.displayName ?? user.loginName ?? ''),
    email: String(user.email ?? ''),
    avatar: String(user.avatar ?? ''),
    organization: String(user.organizationId ?? user.organization ?? 'default'),
    roles,
    permissions,
    admin,
    groups: Array.isArray(user.groups) ? user.groups.map(String) : [],
  }
}

function mapCategory(value: unknown): Category {
  const item = record(value)
  return { id: item.id == null ? '' : item.id, code: String(item.categoryKey ?? item.code ?? ''), name: String(item.name ?? ''), description: String(item.description ?? ''), sort: Number(item.sortOrder ?? item.sort ?? 0), createdAt: item.createdAt, updatedAt: item.updatedAt }
}

function mapTag(value: unknown): Tag {
  const item = record(value)
  return { id: item.id == null ? '' : item.id, code: String(item.tagKey ?? item.code ?? ''), name: String(item.name ?? ''), sort: Number(item.sortOrder ?? item.sort ?? 0), createdAt: item.createdAt, updatedAt: item.updatedAt }
}

function mapPolicy(value: unknown): AccessPolicy {
  const item = record(value)
  return { policyType: String(item.policyType ?? item.type ?? 'EVERYONE') as AccessPolicy['policyType'], value: String(item.value ?? '') }
}

function mapApplication(value: unknown): Application {
  const item = record(value)
  const categoryId = item.categoryId ? item.categoryId : undefined
  const categoryName = String(item.categoryName ?? '')
  const createdAt = String(item.createdAt ?? '')
  const launchType = String(item.launchType ?? item.ssoType ?? 'URL').toUpperCase()
  return {
    id: asId(item.id), code: String(item.code ?? ''), name: String(item.name ?? ''), description: String(item.description ?? ''), icon: String(item.icon ?? ''),
    categoryId, category: categoryId ? { id: categoryId, code: '', name: categoryName } : undefined,
    ssoType: (launchType || 'URL') as Application['ssoType'], owner: String(item.owner ?? item.createdBy ?? ''), department: String(item.department ?? ''),
    status: (String(item.status ?? 'ENABLED').toUpperCase() || 'ENABLED') as Application['status'], sort: Number(item.sortOrder ?? item.sort ?? 0),
    isFeatured: Boolean(item.featured ?? item.isFeatured), healthCheckEnabled: false, healthStatus: 'UNKNOWN',
    tags: listFrom(item.tags).map(mapTag), policies: listFrom(item.policies).map(mapPolicy), isFavorite: Boolean(item.favorite ?? item.isFavorite),
    isNew: createdAt ? Date.now() - Date.parse(createdAt) < 7 * 24 * 60 * 60 * 1000 : false, createdAt, updatedAt: String(item.updatedAt ?? createdAt),
    createdBy: item.createdBy, updatedBy: item.updatedBy, homeUrl: item.homeUrl, launchUrl: item.launchUrl,
    lifecycleStatus: item.lifecycleStatus, configVersion: item.configVersion == null ? undefined : Number(item.configVersion), publishedAt: item.publishedAt, publishedBy: item.publishedBy,
  }
}

function mapIdentityBinding(value: unknown): IdentityBinding | undefined {
  const item = record(value)
  if (!item.id) return undefined
  return { id: asId(item.id), organizationId: asId(item.organizationId), applicationId: asId(item.applicationId), providerKey: String(item.providerKey ?? ''), protocol: String(item.protocol ?? ''), providerApplicationRef: String(item.providerApplicationRef ?? ''), publicClientId: String(item.publicClientId ?? ''), issuer: String(item.issuer ?? ''), redirectUris: listFrom(item, 'redirectUris').map(String), scopes: listFrom(item, 'scopes').map(String), configurationStatus: String(item.configurationStatus ?? ''), verificationStatus: String(item.verificationStatus ?? ''), verifiedAt: item.verifiedAt, verifiedBy: item.verifiedBy, verificationError: String(item.verificationError ?? ''), configVersion: Number(item.configVersion ?? 0), createdAt: item.createdAt, updatedAt: item.updatedAt }
}

function mapVerification(value: unknown): ApplicationVerification {
  const item = record(value)
  return { id: asId(item.id), applicationId: asId(item.applicationId), bindingId: asId(item.bindingId), checkType: String(item.checkType ?? ''), result: String(item.result ?? ''), errorCode: String(item.errorCode ?? ''), evidenceJson: String(item.evidenceJson ?? ''), verifiedBy: String(item.verifiedBy ?? ''), occurredAt: String(item.occurredAt ?? ''), requestId: String(item.requestId ?? '') }
}

function mapEntitlement(value: unknown): ApplicationEntitlement {
  const item = record(value)
  return {
    applicationCode: String(item.applicationCode ?? ''),
    status: String(item.status ?? 'DISABLED').toUpperCase() as ApplicationEntitlement['status'],
    roles: Array.isArray(item.roles) ? item.roles.map(String) : [],
    version: Number(item.version ?? 0),
    updatedAt: item.updatedAt ? String(item.updatedAt) : undefined,
  }
}

function mapAdminUser(value: unknown): AdminUser {
  const item = record(value)
  return {
    id: asId(item.id),
    organizationId: asId(item.organizationId),
    loginName: String(item.loginName ?? ''),
    displayName: String(item.displayName ?? item.loginName ?? ''),
    email: String(item.email ?? ''),
    status: String(item.status ?? 'DISABLED').toUpperCase() as AdminUser['status'],
    identitySource: String(item.identitySource ?? 'LOCAL'),
    roles: Array.isArray(item.roles) ? item.roles.map(String) : [],
    entitlements: listFrom(item, 'entitlements').map(mapEntitlement),
    createdAt: String(item.createdAt ?? ''),
  }
}

function pageOf<T>(items: T[], page = 1, pageSize = items.length || 1, total = items.length): Page<T> { return { items, total, page, pageSize } }

export interface ListApplicationsParams {
  keyword?: string
  categoryId?: string | number
  tagIds?: (string | number)[]
  featured?: boolean
  favorites?: boolean
  page?: number
  pageSize?: number
}

async function fetchPortalApplications(params: ListApplicationsParams = {}, admin = false): Promise<Application[]> {
  const path = admin ? '/admin/portal/applications' : '/portal/applications'
  const data = await apiFetch<unknown>(`${path}${buildQuery({ keyword: params.keyword, categoryId: params.categoryId == null ? undefined : String(params.categoryId), tagId: params.tagIds?.[0] == null ? undefined : String(params.tagIds[0]), favoritesOnly: params.favorites, limit: 500 })}`)
  return listFrom(data, 'applications', 'items').map(mapApplication)
}

// --- 认证 ---

export interface AuthCapabilities {
  authMode: 'oidc' | 'password'
  passwordLoginEnabled: boolean
  casdoorAccountUrl: string
}

const OIDC_REDIRECT_STORAGE_KEY = 'velora.oidc.redirect'

function internalRedirect(value?: string | null): string {
  const candidate = String(value ?? '').trim()
  if (!candidate || !candidate.startsWith('/') || candidate.startsWith('//') || candidate.includes('\\')) return '/'
  try {
    const parsed = new URL(candidate, window.location.origin)
    if (parsed.origin !== window.location.origin) return '/'
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return '/'
  }
}

export async function getAuthCapabilities(): Promise<AuthCapabilities> {
  const data = record(await apiFetch<unknown>('/system/health'))
  const authMode = String(data.authMode ?? 'oidc').toLowerCase() === 'password' ? 'password' : 'oidc'
  return {
    authMode,
    // In OIDC mode this flag may represent the explicitly enabled Casdoor
    // password compatibility flow; production health must still fail closed.
    passwordLoginEnabled: Boolean(data.passwordLoginEnabled),
    casdoorAccountUrl: String(data.casdoorAccountUrl ?? ''),
  }
}

export async function getMe(): Promise<CurrentUser> { const data = await apiFetch<unknown>('/me'); return mapUser(record(data).user ?? data) }
export async function logout(): Promise<{ status: string; federatedLogoutUrl?: string }> {
  const data = await apiFetch<unknown>('/auth/logout', { method: 'POST', body: {} })
  const item = record(data)
  return { status: 'logged_out', federatedLogoutUrl: String(item.federatedLogoutUrl ?? '') || undefined }
}
export function oidcLoginUrl(redirect?: string): string { return `/api/v1/auth/federated/oidc/casdoor/begin${buildQuery({ organization: 'default', redirect })}` }
export async function beginOIDCLogin(redirect?: string): Promise<string> {
  const data = record(await apiFetch<unknown>(`/auth/federated/oidc/casdoor/begin${buildQuery({ organization: 'default' })}`))
  const target = internalRedirect(redirect)
  try {
    window.sessionStorage.setItem(OIDC_REDIRECT_STORAGE_KEY, target)
  } catch {
    // Session storage can be disabled by a browser policy; root is the safe fallback.
  }
  const redirectURL = String(data.redirectUrl ?? '')
  if (!redirectURL || !/^https:\/\//i.test(redirectURL)) throw new Error('OIDC 登录地址无效')
  return redirectURL
}

export async function completeOIDCLogin(provider: string, code: string, state: string): Promise<void> {
  const safeProvider = String(provider || 'casdoor').toLowerCase().replace(/[^a-z0-9._-]/g, '')
  if (!safeProvider || !code || !state) throw new Error('OIDC 回调参数不完整')
  await apiFetch(`/auth/federated/oidc/${encodeURIComponent(safeProvider)}/callback`, { method: 'POST', body: { code, state } })
}

export function consumeOIDCRedirect(): string {
  try {
    const target = internalRedirect(window.sessionStorage.getItem(OIDC_REDIRECT_STORAGE_KEY))
    window.sessionStorage.removeItem(OIDC_REDIRECT_STORAGE_KEY)
    return target
  } catch {
    return '/'
  }
}

export async function loginWithPassword(username: string, password: string, redirect?: string, _turnstileToken?: string): Promise<{ redirect: string; bridgeAction?: string; bridgeTicket?: string }> {
	const data = await apiFetch<{ bridgeAction?: string; bridgeTicket?: string }>('/auth/login', {
		method: 'POST',
		body: { loginName: username, organization: 'default', password, returnPath: internalRedirect(redirect), turnstileToken: _turnstileToken || undefined },
	})
	return { redirect: internalRedirect(redirect), bridgeAction: data?.bridgeAction, bridgeTicket: data?.bridgeTicket }
}

export function sessionBridgeFallbackURL(action: string, returnPath: string, portalOrigin = window.location.origin): string {
	const bridge = new URL(action)
	if (bridge.protocol !== 'https:' || bridge.username || bridge.password || bridge.pathname !== '/_velora/session/bridge' || bridge.search || bridge.hash) {
		throw new Error('统一认证跳转地址无效')
	}
	const target = new URL(internalRedirect(returnPath), portalOrigin)
	target.searchParams.delete('_velora_bridge_nonce')
	if (target.pathname === '/login/oauth/authorize') {
		target.protocol = bridge.protocol
		target.host = bridge.host
	}
	return target.toString()
}

/** Complete the cross-host Casdoor handoff without putting the ticket in a URL. */
export function submitSessionBridge(action: string, ticket: string, returnPath = '/'): void {
	if (!ticket) throw new Error('统一认证跳转地址无效')
	const fallback = sessionBridgeFallbackURL(action, returnPath)
	const form = document.createElement('form')
	form.method = 'post'
	form.action = action
	form.style.display = 'none'
	const input = document.createElement('input')
	input.type = 'hidden'
	input.name = 'ticket'
	input.value = ticket
	form.appendChild(input)
	document.body.appendChild(form)
	form.submit()
	// Some embedded/extension-controlled browsers finish the cross-host POST
	// and set both auth cookies but fail to commit the 303 navigation. A clean,
	// ticket-free continuation makes that state recover automatically. In a
	// normal browser the document unloads first and this timer never runs.
	window.setTimeout(() => window.location.replace(fallback), 1500)
}
export async function getTurnstileConfig(): Promise<{ enabled: boolean; siteKey: string; action: string }> {
  const data = record(await apiFetch<unknown>('/system/health'))
  return {
    enabled: Boolean(data.turnstileEnabled),
    siteKey: String(data.turnstileSiteKey ?? ''),
    action: String(data.turnstileAction ?? 'login'),
  }
}

// --- 用户中心 ---

export interface UserProfile extends CurrentUser { admin: boolean }
export interface SessionDevice { sessionId: string; userAgent: string; ip: string; lastActiveAt: string; expiresAt: string; revokedAt?: string; current: boolean }
export async function getUserProfile(): Promise<UserProfile> { return (await getMe()) as UserProfile }
export function changePassword(oldPassword: string, newPassword: string): Promise<{ status: string; message: string }> { return apiFetch('/auth/password', { method: 'PATCH', body: { currentPassword: oldPassword, newPassword } }).then(() => ({ status: 'updated', message: '密码已更新，请重新登录' })) }
export async function listSessions(): Promise<SessionDevice[]> {
  const data = await apiFetch<unknown>('/admin/sessions?limit=100')
  return listFrom(data, 'sessions').map((item) => ({ sessionId: asId(item.id), userAgent: String(item.userAgent ?? ''), ip: String(item.clientIp ?? item.ip ?? ''), lastActiveAt: String(item.lastSeenAt ?? item.lastActiveAt ?? ''), expiresAt: String(item.expiresAt ?? ''), current: Boolean(item.current) }))
}
export function revokeSession(sessionId: string): Promise<{ revoked: string }> { return apiFetch(`/admin/sessions/${encodeURIComponent(sessionId)}`, { method: 'DELETE' }).then(() => ({ revoked: sessionId })) }
export async function revokeAllSessions(): Promise<{ status: string }> { const sessions = await listSessions(); await Promise.all(sessions.filter((session) => !session.current).map((session) => revokeSession(session.sessionId))); return { status: 'revoked' } }

// --- API Token（替换旧 Integration Token 路由） ---

export interface IntegrationToken { id: string | number; name: string; scopes: string[]; createdBy: string; expiresAt: string | null; lastUsedAt: string | null; revoked: boolean }
function mapToken(value: unknown): IntegrationToken { const item = record(value); return { id: item.id ?? '', name: String(item.name ?? ''), scopes: Array.isArray(item.scopes) ? item.scopes.map(String) : [], createdBy: '', expiresAt: item.expiresAt ?? null, lastUsedAt: item.lastUsedAt ?? null, revoked: false } }
export async function listIntegrationTokens(): Promise<IntegrationToken[]> { const data = await apiFetch<unknown>('/api-tokens'); return listFrom(data, 'tokens', 'items').map(mapToken) }
export async function createIntegrationToken(input: { name: string; scopes: string[]; expiresInDays?: number }): Promise<{ token: string; message: string }> { const data = await apiFetch<unknown>('/api-tokens', { method: 'POST', body: { name: input.name, scopes: input.scopes, expiresDays: input.expiresInDays ?? 0 } }); const item = record(data); return { token: String(item.secret ?? item.token ?? ''), message: '令牌已创建，密钥仅展示一次' } }
export function revokeIntegrationToken(id: string | number): Promise<{ status: string }> { return apiFetch(`/api-tokens/${encodeURIComponent(String(id))}`, { method: 'DELETE' }).then(() => ({ status: 'revoked' })) }

// --- 门户 ---

export async function listApplications(params: ListApplicationsParams = {}): Promise<Page<Application>> { const all = await fetchPortalApplications(params); const page = params.page ?? 1; const pageSize = params.pageSize ?? 24; return pageOf(all.slice((page - 1) * pageSize, page * pageSize), page, pageSize, all.length) }
export async function getApplication(id: string | number): Promise<Application> { const data = await apiFetch<unknown>(`/portal/applications/${encodeURIComponent(String(id))}`); return mapApplication(record(data).application ?? data) }
export async function launchApplication(id: string | number): Promise<LaunchResult> { const data = record(await apiFetch<unknown>(`/portal/applications/${encodeURIComponent(String(id))}/launch`, { method: 'POST', body: {} })); return { type: 'url', url: String(data.launchUrl ?? record(data.application).launchUrl ?? ''), target: '_blank' } }
export async function listRecent(limit = 8): Promise<RecentItem[]> { const data = await apiFetch<unknown>(`/portal/recent${buildQuery({ limit })}`); return listFrom(data, 'applications', 'items').map((item) => { const application = mapApplication(item); return { application, lastVisitedAt: application.updatedAt, visitCount: Number(item.visitCount ?? 0) } }) }
export async function listPopular(limit = 8): Promise<Application[]> { return (await listRecent(limit)).map((item) => item.application) }
export async function listCategories(): Promise<Category[]> { const data = await apiFetch<unknown>('/portal/categories'); return listFrom(data, 'categories', 'items').map(mapCategory) }
export async function listTags(): Promise<Tag[]> { const data = await apiFetch<unknown>('/portal/tags'); return listFrom(data, 'tags', 'items').map(mapTag) }
export async function addFavorite(applicationId: string | number): Promise<{ favorited: boolean }> { await apiFetch('/portal/favorites', { method: 'POST', body: { applicationId: String(applicationId) } }); return { favorited: true } }
export async function removeFavorite(applicationId: string | number): Promise<{ favorited: boolean }> { await apiFetch(`/portal/favorites/${encodeURIComponent(String(applicationId))}`, { method: 'DELETE' }); return { favorited: false } }

// --- 待办与邮件 ---
// Wave 1 后端没有 mail/todo 服务，读取接口返回空状态避免门户访问旧 404；写操作显式失败，避免假成功。
const unavailable = (feature: string): never => { throw new Error(`${feature} 尚未接入当前后端基座`) }
export function listTodos(_params: { status?: 'open' | 'done' | 'all'; limit?: number } = {}): Promise<TodoListResult> { return Promise.resolve({ items: [], openCount: 0 }) }
export function markTodoDone(_id: string | number): Promise<{ done: boolean }> { return Promise.reject(unavailable('待办')) }
export interface BindMailAccountInput { provider: string; email: string; password: string; displayName?: string; imapHost?: string; imapPort?: number; smtpHost?: string; smtpPort?: number }
export interface ListMailMessagesParams { accountId?: string | number; unread?: boolean; starred?: boolean; keyword?: string; page?: number; pageSize?: number }
const emptyMailCapabilities: MailCapabilities = { idle: false, send: false, reply: false, folders: false, star: false, markRead: false }
export function listMailProviders(): Promise<{ profiles: MailProviderProfile[]; capabilities: MailCapabilities }> { return Promise.resolve({ profiles: [], capabilities: emptyMailCapabilities }) }
export function listMailAccounts(): Promise<MailAccount[]> { return Promise.resolve([]) }
export function bindMailAccount(_input: BindMailAccountInput): Promise<MailAccount> { return Promise.reject(unavailable('邮件')) }
export function unbindMailAccount(_id: string | number): Promise<{ deleted: boolean }> { return Promise.reject(unavailable('邮件')) }
export function testMailAccount(_id: string | number): Promise<{ ok: boolean; error?: string }> { return Promise.reject(unavailable('邮件')) }
export function syncMailAccount(_id: string | number): Promise<MailAccount> { return Promise.reject(unavailable('邮件')) }
export function listMailMessages(_params: ListMailMessagesParams = {}): Promise<Page<MailMessage>> { return Promise.resolve(pageOf([], 1, 20)) }
export function getMailMessage(_id: string | number): Promise<{ message: MailMessage; bodyError?: string }> { return Promise.reject(unavailable('邮件')) }
export function setMailRead(_id: string | number, _read: boolean): Promise<{ read: boolean }> { return Promise.reject(unavailable('邮件')) }
export function setMailStar(_id: string | number, _starred: boolean): Promise<{ starred: boolean }> { return Promise.reject(unavailable('邮件')) }
export interface ConvertMailToTodoInput { title?: string; priority?: TodoPriority; kind?: TodoKind; dueAt?: string | null }
export function convertMailToTodo(_id: string | number, _input: ConvertMailToTodoInput): Promise<TodoItem> { return Promise.reject(unavailable('邮件/待办')) }

// --- 管理端门户 ---

export interface AdminApplicationInput { code: string; name: string; description?: string; keywords?: string; icon?: string; categoryId?: string | number | null; homeUrl?: string; launchUrl?: string; ssoType: string; owner?: string; department?: string; status: string; sort?: number; isFeatured?: boolean; tagIds?: (string | number)[]; policies?: { policyType: string; value: string }[] }
function applicationBody(input: AdminApplicationInput, includeCode: boolean): AnyRecord { const body: AnyRecord = { name: input.name, description: input.description ?? '', icon: input.icon ?? '', categoryId: input.categoryId == null ? '' : String(input.categoryId), homeUrl: input.homeUrl ?? '', launchUrl: input.launchUrl ?? '', launchType: input.ssoType || 'URL', status: input.status || 'ENABLED', sortOrder: input.sort ?? 0, featured: input.isFeatured ?? false, tagIds: (input.tagIds ?? []).map(String) }; if (includeCode) body.code = input.code; return body }
export async function adminListApplications(params: ListApplicationsParams = {}): Promise<Page<Application>> { const all = await fetchPortalApplications(params, true); const page = params.page ?? 1; const pageSize = params.pageSize ?? 20; return pageOf(all.slice((page - 1) * pageSize, page * pageSize), page, pageSize, all.length) }
export async function adminCreateApplication(input: AdminApplicationInput): Promise<Application> { const data = await apiFetch<unknown>('/admin/portal/applications', { method: 'POST', body: applicationBody(input, true) }); return mapApplication(record(data).application ?? data) }
export async function adminUpdateApplication(id: string | number, input: AdminApplicationInput): Promise<Application> { const data = await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}`, { method: 'PATCH', body: applicationBody(input, false) }); return mapApplication(record(data).application ?? data) }
export function adminDeleteApplication(id: string | number): Promise<void> { return apiFetch(`/admin/portal/applications/${encodeURIComponent(String(id))}`, { method: 'DELETE' }).then(() => undefined) }

export interface ApplicationRole {
  id: string
  applicationId: string
  roleKey: string
  name: string
  description: string
  riskLevel: 'NORMAL' | 'PRIVILEGED' | 'CRITICAL'
  status: 'ACTIVE' | 'DISABLED'
  configVersion: number
}
function mapApplicationRole(value: unknown): ApplicationRole {
  const item = record(value)
  return { id: asId(item.id), applicationId: asId(item.applicationId), roleKey: String(item.roleKey ?? ''), name: String(item.name ?? ''), description: String(item.description ?? ''), riskLevel: String(item.riskLevel ?? 'NORMAL').toUpperCase() as ApplicationRole['riskLevel'], status: String(item.status ?? 'ACTIVE').toUpperCase() as ApplicationRole['status'], configVersion: Number(item.configVersion ?? 0) }
}
export async function adminListApplicationRoles(id: string | number): Promise<ApplicationRole[]> { const data = await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/roles`); return listFrom(data, 'roles').map(mapApplicationRole) }
export async function adminReplaceApplicationRoles(id: string | number, roles: Array<Pick<ApplicationRole, 'roleKey' | 'name' | 'description' | 'riskLevel' | 'status'>>): Promise<ApplicationRole[]> { const data = await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/roles`, { method: 'PUT', body: { roles } }); return listFrom(data, 'roles').map(mapApplicationRole) }

export interface IdentityOverview { onboardingEnabled: boolean; adminEntryEnabled: boolean; providerKey: string; adminUrlHost: string; issuer: string; connectionStatus: string; pendingApplicationCount: number; automationEnabled: boolean }
export interface ApplicationOnboarding { application: Application; binding?: IdentityBinding; verifications: ApplicationVerification[]; canPublish: boolean; oneTimeClientSecret?: string }
export async function getIdentityOverview(): Promise<IdentityOverview> { const data = record(await apiFetch<unknown>('/admin/identity/overview')); return { onboardingEnabled: Boolean(data.onboardingEnabled), adminEntryEnabled: Boolean(data.adminEntryEnabled), providerKey: String(data.providerKey ?? ''), adminUrlHost: String(data.adminUrlHost ?? ''), issuer: String(data.issuer ?? ''), connectionStatus: String(data.connectionStatus ?? 'UNCONFIGURED'), pendingApplicationCount: Number(data.pendingApplicationCount ?? 0), automationEnabled: Boolean(data.automationEnabled) } }
export async function getIdentityConsoleLink(): Promise<{ url: string; providerKey: string }> { const data = record(await apiFetch<unknown>('/admin/identity/console-link')); return { url: String(data.url ?? ''), providerKey: String(data.providerKey ?? '') } }
export async function getApplicationOnboarding(id: string | number): Promise<ApplicationOnboarding> { const data = record(await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/onboarding`)); return { application: mapApplication(record(data.application)), binding: mapIdentityBinding(data.binding), verifications: listFrom(data, 'verifications').map(mapVerification), canPublish: Boolean(data.canPublish) } }
export interface IdentityBindingInput { providerKey: string; protocol: string; providerApplicationRef: string; publicClientId: string; issuer?: string; redirectUris?: string[]; scopes?: string[]; approvalId?: string; expectedConfigVersion?: number }
export async function upsertApplicationIdentityBinding(id: string | number, input: IdentityBindingInput): Promise<ApplicationOnboarding> { const data = record(await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/identity-binding`, { method: 'PUT', body: input })); return { application: mapApplication(record(data.application)), binding: mapIdentityBinding(data.binding), verifications: [], canPublish: false, oneTimeClientSecret: data.oneTimeClientSecret ? String(data.oneTimeClientSecret) : undefined } }
export async function verifyApplicationIdentity(id: string | number, expectedConfigVersion?: number): Promise<ApplicationOnboarding & { passed: boolean }> { const data = record(await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/verify`, { method: 'POST', body: { expectedConfigVersion } })); return { application: mapApplication(record(data.application)), binding: mapIdentityBinding(data.binding), verifications: listFrom(data, 'verifications').map(mapVerification), canPublish: Boolean(data.passed), passed: Boolean(data.passed) } }
export async function submitApplicationPublish(id: string | number, expectedConfigVersion?: number): Promise<Application> { const data = record(await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/submit-publish`, { method: 'POST', body: { expectedConfigVersion } })); return mapApplication(record(data.application)) }
export async function publishApplication(id: string | number, expectedConfigVersion?: number): Promise<Application> { const data = record(await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/publish`, { method: 'POST', body: { expectedConfigVersion } })); return mapApplication(record(data.application)) }
export async function disableApplication(id: string | number, expectedConfigVersion?: number, approvalId?: string): Promise<Application> { const data = record(await apiFetch<unknown>(`/admin/portal/applications/${encodeURIComponent(String(id))}/disable`, { method: 'POST', body: { expectedConfigVersion, approvalId } })); return mapApplication(record(data.application)) }
export async function adminSetPolicies(id: string | number, policies: { policyType: string; value: string }[]): Promise<unknown> { return apiFetch(`/admin/portal/applications/${encodeURIComponent(String(id))}/policies`, { method: 'PUT', body: { policies } }) }
export async function adminCreateCategory(input: Partial<Category>): Promise<Category> { const data = await apiFetch<unknown>('/admin/portal/categories', { method: 'POST', body: { categoryKey: input.code ?? '', name: input.name ?? '', description: input.description ?? '', sortOrder: input.sort ?? 0, status: 'ACTIVE' } }); return mapCategory(record(data).category ?? data) }
export async function adminUpdateCategory(id: string | number, input: Partial<Category>): Promise<Category> { const data = await apiFetch<unknown>(`/admin/portal/categories/${encodeURIComponent(String(id))}`, { method: 'PATCH', body: { name: input.name ?? '', description: input.description ?? '', sortOrder: input.sort ?? 0, status: 'ACTIVE' } }); return mapCategory(record(data).category ?? data) }
export function adminDeleteCategory(id: string | number): Promise<void> { return apiFetch(`/admin/portal/categories/${encodeURIComponent(String(id))}`, { method: 'DELETE' }).then(() => undefined) }
export async function adminCreateTag(input: Partial<Tag>): Promise<Tag> { const data = await apiFetch<unknown>('/admin/portal/tags', { method: 'POST', body: { tagKey: input.code ?? '', name: input.name ?? '', sortOrder: input.sort ?? 0 } }); return mapTag(record(data).tag ?? data) }
export async function adminUpdateTag(id: string | number, input: Partial<Tag>): Promise<Tag> { const data = await apiFetch<unknown>(`/admin/portal/tags/${encodeURIComponent(String(id))}`, { method: 'PATCH', body: { name: input.name ?? '', sortOrder: input.sort ?? 0 } }); return mapTag(record(data).tag ?? data) }
export function adminDeleteTag(id: string | number): Promise<void> { return apiFetch(`/admin/portal/tags/${encodeURIComponent(String(id))}`, { method: 'DELETE' }).then(() => undefined) }
export async function adminListAuditLogs(params: { page?: number; pageSize?: number; operator?: string; action?: string } = {}): Promise<Page<AuditLog>> { const data = await apiFetch<unknown>(`/admin/audit-logs${buildQuery({ limit: 500 })}`); const items = listFrom(data, 'events', 'items').map((item): AuditLog => ({ id: item.id ?? '', operator: String(item.actorName ?? item.actorId ?? ''), action: String(item.action ?? ''), resource: String(item.resourceType ?? ''), resourceId: String(item.resourceId ?? ''), ip: String(item.clientIp ?? ''), userAgent: '', requestId: String(item.requestId ?? ''), detail: String(item.detailsJson ?? ''), createdAt: String(item.occurredAt ?? '') })).filter((item) => (!params.operator || item.operator.includes(params.operator)) && (!params.action || item.action === params.action)); const page = params.page ?? 1; const pageSize = params.pageSize ?? 20; return pageOf(items.slice((page - 1) * pageSize, page * pageSize), page, pageSize, items.length) }
export async function adminDashboard(): Promise<DashboardStats> { const [apps, categories, tags] = await Promise.all([adminListApplications({ page: 1, pageSize: 500 }), listCategories(), listTags()]); return { applicationCount: apps.total, categoryCount: categories.length, tagCount: tags.length, favoriteCount: apps.items.filter((item) => item.isFavorite).length, totalLaunches: apps.items.reduce((sum, item) => sum + Number((item as any).visitCount ?? 0), 0), enabledAppCount: apps.items.filter((item) => item.status === 'ENABLED').length, disabledAppCount: apps.items.filter((item) => item.status !== 'ENABLED').length } }

export interface CreateAdminUserInput {
  loginName: string
  displayName: string
  email?: string
  password: string
  roles: string[]
  entitlements: Array<{ applicationCode: string; status: 'ACTIVE' | 'DISABLED'; roles: string[] }>
}

export async function adminListUsers(): Promise<AdminUser[]> {
  const data = await apiFetch<unknown>('/admin/users')
  return listFrom(data, 'users', 'items').map(mapAdminUser)
}

export async function adminCreateUser(input: CreateAdminUserInput): Promise<AdminUser> {
  const data = await apiFetch<unknown>('/admin/users', { method: 'POST', body: input })
  return mapAdminUser(record(data).user ?? data)
}

export async function adminUpdateUserStatus(userId: string, status: 'ACTIVE' | 'DISABLED'): Promise<AdminUser> {
  const data = await apiFetch<unknown>(`/admin/users/${encodeURIComponent(userId)}/status`, { method: 'PATCH', body: { status } })
  return mapAdminUser(record(data).user ?? data)
}

export async function adminUpdateUserEntitlement(
  userId: string,
  applicationCode: string,
  status: 'ACTIVE' | 'DISABLED',
  roles: string[],
): Promise<AdminUser> {
  const data = await apiFetch<unknown>(`/admin/users/${encodeURIComponent(userId)}/entitlements/${encodeURIComponent(applicationCode)}`, {
    method: 'PUT',
    body: { status, roles },
  })
  return mapAdminUser(record(data).user ?? data)
}

// Wave 1 没有门户设置表。设置页使用浏览器本地存储作为明确的临时适配，避免继续访问旧 /portal/settings。
const defaultSettings = [{ key: 'portal_name', value: 'Velora' }, { key: 'portal_welcome', value: '企业应用门户' }, { key: 'portal_footer', value: '' }, { key: 'announcement', value: '' }, { key: 'ui_scale', value: '1' }, { key: 'new_badge_days', value: '7' }]
// Wave 1 后端没有共享门户设置服务；固定只读默认值，避免把 localStorage 误当成生产配置。
export function getPortalSettings(): Promise<{ key: string; value: string }[]> { return Promise.resolve(defaultSettings) }
export function updatePortalSetting(_key: string, _value: string): Promise<unknown> { return Promise.reject(unavailable('门户设置服务')) }
export async function getSystemVersion(): Promise<{ application: string; version?: string }> { const data = record(await apiFetch<unknown>('/system/info')); return { application: String(data.service ?? data.application ?? 'Velora'), version: data.version ? String(data.version) : undefined } }

export const queryKeys = { me: ['me'] as const, applications: (params?: unknown) => ['applications', params] as const, application: (id: string | number) => ['applications', id] as const, recent: ['recent'] as const, popular: ['popular'] as const, categories: ['categories'] as const, tags: ['tags'] as const, favorites: ['favorites'] as const, todos: ['todos'] as const, mailAccounts: ['mail', 'accounts'] as const, mailProviders: ['mail', 'providers'] as const, mailMessages: (params?: unknown) => ['mail', 'messages', params] as const, mailMessage: (id: string | number) => ['mail', 'messages', 'detail', id] as const, adminApplications: (params?: unknown) => ['admin', 'applications', params] as const, applicationRoles: (id: string | number) => ['admin', 'applications', id, 'roles'] as const, adminUsers: ['admin', 'users'] as const, auditLogs: (params?: unknown) => ['admin', 'audit-logs', params] as const, dashboard: ['admin', 'dashboard'] as const, identityOverview: ['admin', 'identity', 'overview'] as const, applicationOnboarding: (id: string | number) => ['admin', 'identity', 'onboarding', id] as const, portalSettings: ['portal', 'settings'] as const }
