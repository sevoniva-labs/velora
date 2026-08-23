// Velora 领域类型（与后端 OpenAPI/JSON 契约一致，snake_case 响应已由 client 解包）。

// OIDC：目标应用直接对接 Casdoor；Velora 只负责门户侧的统一登录入口。
export type SSOType = 'URL' | 'OIDC' | 'SAML' | 'CAS' | 'FORWARD_AUTH' | 'VELORA_OIDC'
export type AppStatus = 'ENABLED' | 'DISABLED'
export type HealthStatus = 'UP' | 'DOWN' | 'UNKNOWN'
export type PolicyType = 'EVERYONE' | 'ORGANIZATION' | 'ROLE' | 'GROUP' | 'USER'

export interface CurrentUser {
  id: string
  username: string
  displayName: string
  email: string
  avatar: string
  organization: string
  roles: string[]
  /** 服务端授权模型计算出的权限集合；界面只据此决定管理入口。 */
  permissions: string[]
  /** 兼容旧页面的管理员标记，来源必须是 permissions。 */
  admin?: boolean
  groups: string[]
}

export interface ApplicationEntitlement {
  applicationCode: string
  status: 'ACTIVE' | 'DISABLED'
  roles: string[]
  version: number
  updatedAt?: string
}

export interface AdminUser {
  id: string
  organizationId: string
  loginName: string
  displayName: string
  email: string
  status: 'ACTIVE' | 'DISABLED' | 'LOCKED'
  identitySource: string
  roles: string[]
  entitlements: ApplicationEntitlement[]
  createdAt: string
}

export interface Department {
  id: string
  organizationId: string
  parentId: string
  departmentKey: string
  name: string
  status: 'ACTIVE' | 'DISABLED'
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface Position {
  id: string
  organizationId: string
  departmentId: string
  positionKey: string
  name: string
  description: string
  status: 'ACTIVE' | 'DISABLED'
  sortOrder: number
}

export interface UserGroup {
  id: string
  organizationId: string
  groupKey: string
  name: string
  description: string
  status: 'ACTIVE' | 'DISABLED'
  roles: string[]
  memberIds: string[]
  memberCount: number
}

export interface PlatformRole {
  key: string
  name: string
  description: string
  dataScope: string
  permissions: string[]
  dataScopeDepartmentIds: string[]
}

export interface PlatformPermission {
  key: string
  name: string
  description: string
  resource: string
  action: string
}

export interface AdminSession {
  id: string
  userId: string
  loginName: string
  clientIp: string
  userAgent: string
  createdAt: string
  expiresAt: string
  lastSeenAt: string
  current: boolean
}

export interface UserAssignment {
  id?: string
  departmentId: string
  positionId?: string
  primary: boolean
  validFrom?: string
  validUntil?: string
}

export interface ApplicationAccessGrant {
  id: string
  applicationId: string
  subjectType: 'EVERYONE' | 'DEPARTMENT' | 'USER_GROUP' | 'PLATFORM_ROLE' | 'USER'
  subjectId: string
  subjectName: string
  includeDescendants: boolean
  effect: 'ALLOW' | 'EXCLUDE'
  roles: string[]
  validFrom?: string
  validUntil?: string
  status: 'ACTIVE' | 'DISABLED'
  reason: string
  version: number
  createdAt?: string
  updatedAt?: string
}

export interface ApplicationAccessImpact {
  effectiveUsers: number
  addedUsers: number
  revokedUsers: number
  roleChangedUsers: number
  privilegedUsers: number
  provisioningTasks: number
}

export interface ApplicationEffectiveAccess {
  userId: string
  loginName: string
  displayName: string
  roles: string[]
  sourceGrantIds: string[]
}

export interface Category {
  id: string | number
  code: string
  name: string
  description: string
  sort: number
  createdAt?: string
  updatedAt?: string
}

export interface Tag {
  id: string | number
  code: string
  name: string
  sort: number
  createdAt?: string
  updatedAt?: string
}

export interface AccessPolicy {
  policyType: PolicyType
  value: string
}

export interface Application {
  id: string | number
  code: string
  name: string
  description: string
  keywords?: string
  icon: string
  categoryId?: string | number
  category?: { id: string | number; code: string; name: string }
  ssoType: SSOType
  owner: string
  department: string
  status: AppStatus
  sort: number
  isFeatured: boolean
  healthCheckEnabled: boolean
  healthStatus?: HealthStatus
  tags: Tag[]
  policies?: AccessPolicy[]
  isFavorite?: boolean
  isNew?: boolean
  createdAt: string
  updatedAt: string
  createdBy?: string
  updatedBy?: string
  lifecycleStatus?: string
  configVersion?: number
  publishedAt?: string
  publishedBy?: string
  // 仅管理员视图：
  homeUrl?: string
  launchUrl?: string
  healthCheckUrl?: string
}

export interface IdentityBinding {
  id: string
  organizationId: string
  applicationId: string
  providerKey: string
  protocol: string
  providerApplicationRef: string
  publicClientId: string
  issuer: string
  redirectUris: string[]
  scopes: string[]
  configurationStatus: string
  verificationStatus: string
  verifiedAt?: string
  verifiedBy?: string
  verificationError: string
  configVersion: number
  createdAt?: string
  updatedAt?: string
}

export interface ApplicationVerification {
  id: string
  applicationId: string
  bindingId: string
  checkType: string
  result: string
  errorCode: string
  evidenceJson: string
  verifiedBy: string
  occurredAt: string
  requestId: string
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface RecentItem {
  application: Application
  lastVisitedAt: string
  visitCount: number
}

export interface LaunchResult {
  type: 'url' | 'redirect'
  url: string
  target: '_self' | '_blank'
}

/** 待办事项（待办中心）：支持外部系统通过 API 集成，sourceSystem+sourceId 为幂等键。 */
export type TodoPriority = 'urgent' | 'high' | 'mid' | 'low'

/** 待办类型（Tab 维度） */
export type TodoKind = 'mail' | 'approval' | 'devops' | 'ops' | 'project' | 'hr' | 'other'

export interface TodoItem {
  id: number
  userId: string
  title: string
  kind: TodoKind
  sourceSystem: string
  sourceLabel: string
  sourceId: string
  priority: TodoPriority
  url: string
  dueAt?: string | null
  status: 'open' | 'done'
  createdAt: string
  updatedAt: string
}

export interface TodoListResult {
  items: TodoItem[]
  openCount: number
}

/** 邮件账号（credential 为密文，后端永不返回明文）。 */
export interface MailAccount {
  id: number
  userId: string
  provider: string
  email: string
  displayName: string
  authType: string
  imapHost: string
  imapPort: number
  smtpHost: string
  smtpPort: number
  status: 'active' | 'error' | 'disabled'
  syncEnabled: boolean
  unreadCount: number
  lastSyncAt?: string | null
  lastError: string
  createdAt: string
  updatedAt: string
}

export interface MailMessage {
  id: number
  accountId: number
  userId: string
  folder: string
  uid: number
  messageId: string
  subject: string
  fromAddress: string
  fromName: string
  toAddresses: string
  receivedAt?: string | null
  isRead: boolean
  isStarred: boolean
  hasAttachment: boolean
  snippet: string
  bodyText?: string
  bodyHtml?: string
  size: number
  createdAt: string
  updatedAt: string
}

export interface MailProviderProfile {
  provider: string
  label: string
  imapHost: string
  imapPort: number
  smtpHost: string
  smtpPort: number
}

export interface MailCapabilities {
  idle: boolean
  send: boolean
  reply: boolean
  folders: boolean
  star: boolean
  markRead: boolean
}

export interface AuditLog {
  id: number
  operator: string
  action: string
  resource: string
  resourceId: string
  ip: string
  userAgent: string
  requestId: string
  detail: string
  createdAt: string
}

export interface DashboardStats {
  applicationCount: number
  categoryCount: number
  tagCount: number
  favoriteCount: number
  totalLaunches: number
  enabledAppCount: number
  disabledAppCount: number
}
