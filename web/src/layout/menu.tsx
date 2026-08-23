import type { ReactNode } from 'react'
import {
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  DashboardOutlined,
  SafetyCertificateOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { API_TOKEN_MANAGE, AUDIT_READ, PORTAL_MANAGE, SYSTEM_USER_READ } from '../auth/permissions'

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
    key: 'admin-app-center', label: '应用中心', icon: <AppstoreOutlined />, permissions: [PORTAL_MANAGE],
    children: [{ key: 'admin-apps', path: '/admin/applications', label: '应用', permissions: [PORTAL_MANAGE] }],
  },
  {
    key: 'admin-people', label: '组织与人员', icon: <TeamOutlined />, permissions: [SYSTEM_USER_READ],
    children: [{ key: 'admin-users', path: '/admin/users', label: '用户', permissions: [SYSTEM_USER_READ] }],
  },
  {
    key: 'admin-security', label: '安全与审计', icon: <SafetyCertificateOutlined />, permissions: [AUDIT_READ, API_TOKEN_MANAGE],
    children: [
      { key: 'admin-integration-tokens', path: '/admin/integration-tokens', label: '服务账号', permissions: [API_TOKEN_MANAGE], icon: <ApiOutlined /> },
      { key: 'admin-audit', path: '/admin/audit', label: '操作审计', permissions: [AUDIT_READ], icon: <AuditOutlined /> },
    ],
  },
]

export function adminActiveKey(pathname: string): string {
  if (pathname.startsWith('/admin/users')) return 'admin-users'
  if (pathname.startsWith('/admin/applications')) return 'admin-apps'
  if (pathname.startsWith('/admin/audit')) return 'admin-audit'
  if (pathname.startsWith('/admin/integration-tokens')) return 'admin-integration-tokens'
  return 'admin-overview'
}
