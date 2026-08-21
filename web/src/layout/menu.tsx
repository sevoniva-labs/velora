// 管理后台左侧菜单定义（与 admin 路由一一对应）。
import type { ReactNode } from 'react'
import {
  ApiOutlined,
  AppstoreOutlined,
  AuditOutlined,
  DashboardOutlined,
  SafetyCertificateOutlined,
  TagsOutlined,
  UnorderedListOutlined,
  SafetyOutlined,
} from '@ant-design/icons'

export interface AdminNavItem {
  key: string
  path: string
  label: string
  icon?: ReactNode
  permission?: string
}

export interface AdminNavGroup {
  key: string
  label: string
  items: AdminNavItem[]
}

export const adminNavGroups: AdminNavGroup[] = [
  {
    key: 'workspace',
    label: '门户管理',
    items: [
      { key: 'admin-overview', path: '/admin', label: '概览', icon: <DashboardOutlined /> },
      { key: 'admin-apps', path: '/admin/applications', label: '应用管理', icon: <AppstoreOutlined />, permission: 'portal.application.manage' },
      { key: 'admin-categories', path: '/admin/categories', label: '分类管理', icon: <UnorderedListOutlined /> },
      { key: 'admin-tags', path: '/admin/tags', label: '标签管理', icon: <TagsOutlined /> },
      { key: 'admin-policies', path: '/admin/policies', label: '访问策略', icon: <SafetyCertificateOutlined /> },
    ],
  },
  {
    key: 'platform',
    label: '平台',
    items: [
      { key: 'admin-audit', path: '/admin/audit', label: '审计日志', icon: <AuditOutlined /> },
      { key: 'admin-integration-tokens', path: '/admin/integration-tokens', label: '集成令牌', icon: <ApiOutlined /> },
      { key: 'admin-identity', path: '/admin/identity', label: '身份与单点登录', icon: <SafetyOutlined />, permission: 'iam.integration.read' },
    ],
  },
]

export function adminActiveKey(pathname: string): string {
  if (pathname.startsWith('/admin/applications')) return 'admin-apps'
  if (pathname.startsWith('/admin/categories')) return 'admin-categories'
  if (pathname.startsWith('/admin/tags')) return 'admin-tags'
  if (pathname.startsWith('/admin/policies')) return 'admin-policies'
  if (pathname.startsWith('/admin/audit')) return 'admin-audit'
  if (pathname.startsWith('/admin/integration-tokens')) return 'admin-integration-tokens'
  if (pathname.startsWith('/admin/identity')) return 'admin-identity'
  return 'admin-overview'
}
