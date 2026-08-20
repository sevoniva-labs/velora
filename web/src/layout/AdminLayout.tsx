// 管理后台外壳：顶栏（品牌 + 返回门户 + 头像）+ 左侧菜单（访问策略/审计等）。
// 与普通门户保持同一套品牌语言。
import { useState, type ReactNode } from 'react'
import { App as AntdApp, Avatar, Button, Drawer, Dropdown, Layout, Menu } from 'antd'
import { HomeOutlined, LogoutOutlined, MenuFoldOutlined, MenuOutlined, MenuUnfoldOutlined, UserOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getPortalSettings, queryKeys } from '../api/api'
import { useMe } from '../auth/useMe'
import { logout } from '../api/api'
import { adminActiveKey, adminNavGroups } from './menu'

export interface AdminLayoutProps {
  children: ReactNode
}

export function AdminLayout({ children }: AdminLayoutProps) {
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const { message } = AntdApp.useApp()
  const location = useLocation()
  const navigate = useNavigate()
  const me = useMe()
  const queryClient = useQueryClient()

  const { data: settings } = useQuery({ queryKey: queryKeys.portalSettings, queryFn: getPortalSettings })
  const portalName = settings?.find((s) => s.key === 'portal_name')?.value || 'Velora'
  const portalWelcome = settings?.find((s) => s.key === 'portal_welcome')?.value || '企业应用门户'

  const displayName = me.data?.displayName || me.data?.username || '用户'
  const activeKey = adminActiveKey(location.pathname)

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: (result) => {
      queryClient.clear()
      // 整页跳转：让浏览器应用服务端下发的清除 Cookie，并彻底重置前端状态。
      window.location.assign(result.federatedLogoutUrl || '/login')
    },
    onError: (err) => {
      message.error(err instanceof Error ? err.message : '退出失败，请稍后再试')
    },
  })

  const menuItems = adminNavGroups.map((g) => ({
    type: 'group' as const,
    key: g.key,
    label: g.label,
    children: g.items.map((item) => ({
      key: item.key,
      icon: item.icon,
      label: (
        <Link className="velora-side-link" to={item.path}>
          {item.label}
        </Link>
      ),
    })),
  }))
  const flatItems = adminNavGroups.flatMap((g) => g.items)

  return (
    <Layout className={collapsed ? 'velora-layout is-collapsed' : 'velora-layout'}>
      <Layout.Header className="velora-header">
        <Button
          type="text"
          className="velora-header-trigger velora-mobile-menu-trigger"
          aria-label="打开导航"
          icon={<MenuOutlined />}
          onClick={() => setMobileOpen(true)}
        />
        <div className="velora-header-brand">
          <Link className="velora-brand" to="/admin" aria-label="管理后台">
            <span className="velora-brand-mark" aria-hidden="true">
              <img src="/sevoniva-mark.svg" alt="" width={19} height={19} style={{ display: 'block' }} />
            </span>
            <span className="velora-brand-text">
              <span className="velora-brand-name">{portalName}</span>
              <span className="velora-brand-sub">{portalWelcome} · 管理后台</span>
            </span>
          </Link>
        </div>
        <Button
          type="text"
          className="velora-header-trigger velora-sider-trigger"
          aria-label={collapsed ? '展开' : '折叠'}
          icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          onClick={() => setCollapsed((v) => !v)}
        />
        <div className="velora-header-toolbar">
          <Button className="velora-back-portal" icon={<HomeOutlined />} onClick={() => navigate('/home')}>
            返回门户
          </Button>
          <span className="velora-header-divider" aria-hidden="true" />
          <Dropdown
            menu={{
              items: [
                {
                  key: 'sign-out',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: () => logoutMutation.mutate(),
                },
              ],
            }}
            trigger={['click']}
          >
            <button type="button" className="velora-user-chip" aria-label={`当前用户：${displayName}`}>
              <Avatar size={28} icon={<UserOutlined />} />
              <span className="velora-user-chip-info">
                <span className="velora-user-chip-name">{displayName}</span>
                <span className="velora-user-chip-role">管理员</span>
              </span>
            </button>
          </Dropdown>
        </div>
      </Layout.Header>

      <Drawer
        placement="left"
        width={220}
        open={mobileOpen}
        title="Velora 管理后台"
        styles={{ body: { padding: 0 } }}
        onClose={() => setMobileOpen(false)}
      >
        <Menu
          mode="inline"
          selectable={false}
          selectedKeys={[activeKey]}
          items={menuItems}
          style={{ borderInlineEnd: 'none' }}
          onClick={({ key }) => {
            setMobileOpen(false)
            const item = flatItems.find((i) => i.key === key)
            if (item) navigate(item.path)
          }}
        />
      </Drawer>

      <Layout hasSider className="velora-body">
        <Layout.Sider
          width={220}
          collapsedWidth={64}
          collapsible
          collapsed={collapsed}
          trigger={null}
          theme="light"
          className="velora-sider"
        >
          <div className="velora-sider-inner">
            <Menu
              mode="inline"
              inlineCollapsed={collapsed}
              selectable={false}
              selectedKeys={[activeKey]}
              items={menuItems}
              className="velora-side-menu"
            />
          </div>
        </Layout.Sider>
        <Layout.Content id="main" className="velora-main-content">
          <div className="velora-page-content velora-admin-content">{children}</div>
        </Layout.Content>
      </Layout>
    </Layout>
  )
}
