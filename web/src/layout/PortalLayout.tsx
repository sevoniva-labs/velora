// 普通用户门户外壳：技术蓝顶栏（品牌 + 导航 + 搜索 + 头像）+ 居中内容区。
// 复用 Spectra Web 的视觉体系（velora-* 样式类）。
import type { ReactNode } from 'react'
import { App as AntdApp, Avatar, Button, Dropdown, Layout, Space, Tag } from 'antd'
import { LogoutOutlined, SettingOutlined, UserOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useMe } from '../auth/useMe'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { logout } from '../api/api'

export interface PortalLayoutProps {
  children: ReactNode
}

const NAV_ITEMS = [
  { key: 'home', path: '/home', label: '首页' },
  { key: 'apps', path: '/applications', label: '应用中心' },
  { key: 'favorites', path: '/favorites', label: '我的收藏' },
]

export function PortalLayout({ children }: PortalLayoutProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { message } = AntdApp.useApp()
  const me = useMe()

  const isAdmin = (me.data?.roles ?? []).includes('velora_admin')
  const displayName = me.data?.displayName || me.data?.username || '用户'
  const activeKey = NAV_ITEMS.find((i) => location.pathname.startsWith(i.path))?.key ?? 'home'

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSettled: () => {
      queryClient.clear()
      message.success('已退出登录')
      navigate('/login', { replace: true })
    },
  })

  return (
    <Layout className="velora-layout">
      <Layout.Header className="velora-header">
        <div className="velora-header-brand">
          <Link className="velora-brand" to="/home" aria-label="首页">
            <span className="velora-brand-mark" aria-hidden="true">
              <img src="/logo-mark.svg" alt="" width={18} height={18} style={{ display: 'block' }} />
            </span>
            <span className="velora-brand-name">Velora</span>
          </Link>
        </div>

        <nav className="velora-module-nav" aria-label="导航">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.key}
              type="button"
              className={activeKey === item.key ? 'velora-module-tab is-active' : 'velora-module-tab'}
              onClick={() => navigate(item.path)}
            >
              {item.label}
            </button>
          ))}
          {isAdmin && (
            <button
              type="button"
              className={location.pathname.startsWith('/admin') ? 'velora-module-tab is-active' : 'velora-module-tab'}
              onClick={() => navigate('/admin')}
            >
              管理后台
            </button>
          )}
        </nav>

        <div className="velora-header-toolbar">
          <Space size={6}>
            {isAdmin && (
              <Link to="/admin" aria-label="管理后台">
                <Tag color="blue" style={{ marginInlineEnd: 0 }}>管理员</Tag>
              </Link>
            )}
          </Space>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'admin',
                  icon: <SettingOutlined />,
                  label: '管理后台',
                  disabled: !isAdmin,
                  onClick: () => navigate('/admin'),
                },
                { type: 'divider' },
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
            <Button
              type="text"
              className="velora-user-actions"
              aria-label={displayName}
              icon={<Avatar size={28} icon={<UserOutlined />} />}
            />
          </Dropdown>
        </div>
      </Layout.Header>

      <Layout.Content className="velora-main-content velora-home-main">
        <div className="velora-home-page-content">{children}</div>
      </Layout.Content>
    </Layout>
  )
}
