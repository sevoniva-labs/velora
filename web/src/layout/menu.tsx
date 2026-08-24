import type { ReactNode } from 'react'
import {
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  CheckSquareOutlined,
  ClockCircleOutlined,
  DashboardOutlined,
  ApartmentOutlined,
  LockOutlined,
  SafetyOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
  UsergroupAddOutlined,
} from '@ant-design/icons'
import { API_TOKEN_MANAGE, APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE, AUDIT_READ, IDENTITY_MANAGE, IDENTITY_READ, IDENTITY_VERIFY, PORTAL_MANAGE, PORTAL_PUBLISH, SYSTEM_ACCESS_REVIEW_READ, SYSTEM_CONFIG_READ, SYSTEM_DEPARTMENT_READ, SYSTEM_ORGANIZATION_READ, SYSTEM_POSITION_READ, SYSTEM_ROLE_READ, SYSTEM_SESSION_READ, SYSTEM_TEMPORARY_GRANT_READ, SYSTEM_USER_GROUP_READ, SYSTEM_USER_READ } from '../auth/permissions'

export interface AdminNavItem {
  key: string
  path?: string
  label: string
  icon?: ReactNode
  permissions?: string[]
  children?: AdminNavItem[]
}

export const adminNavItems: AdminNavItem[] = [
  { key: 'admin-overview', path: '/admin', label: '工作台', icon: <DashboardOutlined />, permissions: [PORTAL_MANAGE, AUDIT_READ, SYSTEM_USER_READ] },
  {
    key: 'admin-app-center', label: '应用中心', icon: <AppstoreOutlined />, permissions: [PORTAL_MANAGE, PORTAL_PUBLISH, IDENTITY_READ, IDENTITY_MANAGE, IDENTITY_VERIFY],
    children: [
      { key: 'admin-apps', path: '/admin/applications', label: '应用', permissions: [PORTAL_MANAGE, PORTAL_PUBLISH, IDENTITY_READ, IDENTITY_MANAGE, IDENTITY_VERIFY] },
      { key: 'admin-taxonomy', path: '/admin/taxonomy', label: '应用分类', permissions: [PORTAL_MANAGE] },
    ],
  },
  {
    key: 'admin-people', label: '组织与人员', icon: <TeamOutlined />, permissions: [SYSTEM_USER_READ, SYSTEM_ORGANIZATION_READ, SYSTEM_DEPARTMENT_READ, SYSTEM_POSITION_READ, SYSTEM_USER_GROUP_READ],
    children: [
      { key: 'admin-users', path: '/admin/users', label: '用户', permissions: [SYSTEM_USER_READ] },
      { key: 'admin-organization', path: '/admin/organization', label: '组织架构', permissions: [SYSTEM_ORGANIZATION_READ, SYSTEM_DEPARTMENT_READ, SYSTEM_POSITION_READ], icon: <ApartmentOutlined /> },
      { key: 'admin-user-groups', path: '/admin/user-groups', label: '用户组', permissions: [SYSTEM_USER_GROUP_READ], icon: <UsergroupAddOutlined /> },
    ],
  },
  {
    key: 'admin-governance', label: '权限治理', icon: <SafetyOutlined />, permissions: [SYSTEM_ROLE_READ, APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE, SYSTEM_TEMPORARY_GRANT_READ, SYSTEM_ACCESS_REVIEW_READ],
    children: [
      { key: 'admin-roles', path: '/admin/roles', label: '平台角色', permissions: [SYSTEM_ROLE_READ] },
      { key: 'admin-approvals', path: '/admin/approvals', label: '审批', permissions: [APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE], icon: <CheckSquareOutlined /> },
      { key: 'admin-temporary-grants', path: '/admin/temporary-grants', label: '临时授权', permissions: [SYSTEM_TEMPORARY_GRANT_READ], icon: <ClockCircleOutlined /> },
      { key: 'admin-access-reviews', path: '/admin/access-reviews', label: '权限检查', permissions: [SYSTEM_ACCESS_REVIEW_READ], icon: <AuditOutlined /> },
    ],
  },
  {
    key: 'admin-security', label: '安全与审计', icon: <SafetyCertificateOutlined />, permissions: [AUDIT_READ, API_TOKEN_MANAGE, SYSTEM_SESSION_READ, SYSTEM_CONFIG_READ],
    children: [
      { key: 'admin-login-security', path: '/admin/login-security', label: '登录安全', permissions: [SYSTEM_CONFIG_READ], icon: <SafetyCertificateOutlined /> },
      { key: 'admin-integration-tokens', path: '/admin/integration-tokens', label: '系统对接', permissions: [API_TOKEN_MANAGE], icon: <ApiOutlined /> },
      { key: 'admin-sessions', path: '/admin/sessions', label: '登录会话', permissions: [SYSTEM_SESSION_READ], icon: <LockOutlined /> },
      { key: 'admin-audit', path: '/admin/audit', label: '操作记录', permissions: [AUDIT_READ], icon: <AuditOutlined /> },
    ],
  },
  {
    key: 'admin-settings', label: '平台设置', icon: <SettingOutlined />, permissions: [SYSTEM_CONFIG_READ],
    children: [{ key: 'admin-config-changes', path: '/admin/config-changes', label: '配置变更', permissions: [SYSTEM_CONFIG_READ] }],
  },
]

export function adminActiveKey(pathname: string): string {
  if (pathname.startsWith('/admin/users')) return 'admin-users'
  if (pathname.startsWith('/admin/organization')) return 'admin-organization'
  if (pathname.startsWith('/admin/user-groups')) return 'admin-user-groups'
  if (pathname.startsWith('/admin/roles')) return 'admin-roles'
  if (pathname.startsWith('/admin/approvals')) return 'admin-approvals'
  if (pathname.startsWith('/admin/temporary-grants')) return 'admin-temporary-grants'
  if (pathname.startsWith('/admin/access-reviews')) return 'admin-access-reviews'
  if (pathname.startsWith('/admin/login-security')) return 'admin-login-security'
  if (pathname.startsWith('/admin/sessions')) return 'admin-sessions'
  if (pathname.startsWith('/admin/applications')) return 'admin-apps'
  if (pathname.startsWith('/admin/taxonomy')) return 'admin-taxonomy'
  if (pathname.startsWith('/admin/audit')) return 'admin-audit'
  if (pathname.startsWith('/admin/integration-tokens')) return 'admin-integration-tokens'
  if (pathname.startsWith('/admin/config-changes')) return 'admin-config-changes'
  return 'admin-overview'
}
