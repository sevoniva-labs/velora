import { useMemo, useState, type ReactNode } from 'react'
import { ProLayout, type MenuDataItem } from '@ant-design/pro-components'
import { Avatar, Button, Dropdown } from 'antd'
import { HomeOutlined, LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { logout } from '../api/api'
import { useMe } from '../auth/useMe'
import { adminNavItems, type AdminNavItem } from './menu'
import { hasAnyPermission } from '../auth/permissions'
import { portalConfig } from '../config/portal'

export interface AdminLayoutProps { children: ReactNode }

const ROLE_LABELS: Record<string, string> = {
  system_admin: '系统管理员', application_admin: '应用管理员', iam_admin: '身份管理员', auditor: '审计员', user: '普通用户',
}

export function visibleNavigation(items: AdminNavItem[], permissions: string[], roles: string[]): AdminNavItem[] {
  return items.flatMap((item) => {
    const children = item.children ? visibleNavigation(item.children, permissions, roles) : undefined
    const ownVisible = !item.permissions?.length || hasAnyPermission(permissions, item.permissions, roles)
    if (!ownVisible && !children?.length) return []
    return [{ ...item, children }]
  })
}

function toMenuData(items: AdminNavItem[]): MenuDataItem[] {
  return items.map((item) => ({ key: item.key, path: item.path, name: item.label, icon: item.icon, children: item.children ? toMenuData(item.children) : undefined }))
}

export function AdminLayout({ children }: AdminLayoutProps) {
  const [collapsed, setCollapsed] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const me = useMe()
  const queryClient = useQueryClient()
  const portalName = portalConfig.name
  const displayName = me.data?.displayName || me.data?.username || '用户'
  const roleLabel = me.data?.roles.map((role) => ROLE_LABELS[role]).find(Boolean) || '管理成员'
  const menuData = useMemo(() => toMenuData(visibleNavigation(adminNavItems, me.data?.permissions ?? [], me.data?.roles ?? [])), [me.data?.permissions, me.data?.roles])

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: (result) => { queryClient.clear(); window.location.assign(result.federatedLogoutUrl || '/login') },
  })

  return (
    <ProLayout
      className="velora-admin-pro-layout"
      layout="side"
      title={portalName}
      logo="/sevoniva-mark.svg"
      location={{ pathname: location.pathname }}
      menuDataRender={() => menuData}
      menuItemRender={(item, dom) => item.path ? <Link to={item.path}>{dom}</Link> : dom}
      collapsed={collapsed}
      onCollapse={setCollapsed}
      siderWidth={208}
      fixedHeader
      fixSiderbar
      breakpoint="lg"
      token={{
        header: { colorBgHeader: 'var(--admin-bg-canvas)', colorHeaderTitle: 'var(--admin-text-primary)' },
        sider: { colorMenuBackground: 'var(--admin-bg-canvas)', colorTextMenu: 'var(--admin-text-secondary)', colorTextMenuSelected: 'var(--admin-text-primary)', colorBgMenuItemSelected: 'var(--admin-menu-active)' },
        pageContainer: { paddingInlinePageContainerContent: 0, paddingBlockPageContainerContent: 0 },
      }}
      actionsRender={() => [<Button key="portal" type="text" icon={<HomeOutlined />} onClick={() => navigate('/home')}>返回门户</Button>]}
      avatarProps={{
        src: <Avatar size={28} icon={<UserOutlined />} />,
        title: displayName,
        render: (_, dom) => <Dropdown trigger={['click']} menu={{ items: [
          { key: 'role', label: roleLabel, disabled: true },
          { type: 'divider' },
          { key: 'logout', label: '退出登录', icon: <LogoutOutlined />, danger: true, onClick: () => logoutMutation.mutate() },
        ] }}>{dom}</Dropdown>,
      }}
      contentStyle={{ padding: 24, minHeight: 'calc(100vh - 56px)' }}
    >
      <main id="main" className="velora-admin-content">{children}</main>
    </ProLayout>
  )
}
