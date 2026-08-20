// Velora 领域类型（与后端 OpenAPI/JSON 契约一致，snake_case 响应已由 client 解包）。

// VELORA_OIDC：通过 Velora 自身 OIDC Provider 登录（统一登录入口，Casdoor 隐藏在后）。
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
  /** 服务端按 VELORA_ADMIN_ROLE 计算的管理员标记 */
  admin?: boolean
  groups: string[]
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
  // 仅管理员视图：
  homeUrl?: string
  launchUrl?: string
  casdoorApplicationName?: string
  casdoorClientId?: string
  healthCheckUrl?: string
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
