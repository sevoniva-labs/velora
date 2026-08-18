// Velora 领域类型（与后端 OpenAPI/JSON 契约一致，snake_case 响应已由 client 解包）。

export type SSOType = 'URL' | 'OIDC' | 'SAML' | 'CAS' | 'FORWARD_AUTH'
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
  groups: string[]
}

export interface Category {
  id: number
  code: string
  name: string
  description: string
  sort: number
  createdAt?: string
  updatedAt?: string
}

export interface Tag {
  id: number
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
  id: number
  code: string
  name: string
  description: string
  keywords?: string
  icon: string
  categoryId?: number
  category?: { id: number; code: string; name: string }
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
