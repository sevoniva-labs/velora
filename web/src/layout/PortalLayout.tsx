// 普通用户门户外壳：深海军蓝顶栏（品牌 + 导航 + 搜索 + 用户）+ 居中内容区。
import type { ReactNode } from 'react'
import { App as AntdApp, Avatar, Dropdown, Input, Layout } from 'antd'
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

  const isAdmin = me.data?.admin === true
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

  const onHeaderSearch = (value: string) => {
    const q = value.trim()
    navigate(q ? `/applications?keyword=${encodeURIComponent(q)}` : '/applications')
  }

  return (
    <Layout className="velora-layout">
      <Layout.Header className="velora-header">
        <div className="velora-header-brand">
          <Link className="velora-brand" to="/home" aria-label="首页">
            <span className="velora-brand-mark" aria-hidden="true">
              <img src="/logo-mark.svg" alt="" width={19} height={19} style={{ display: 'block' }} />
            </span>
            <span className="velora-brand-text">
              <span className="velora-brand-name">Velora</span>
              <span className="velora-brand-sub">企业应用门户</span>
            </span>
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
          <div className="velora-header-search">
            <Input.Search
              size="middle"
              allowClear
              placeholder="搜索应用…"
              onSearch={onHeaderSearch}
              aria-label="搜索应用"
            />
          </div>
          <Dropdown
            menu={{
              items: [
                ...(isAdmin
                  ? [
                      {
                        key: 'admin',
                        icon: <SettingOutlined />,
                        label: '管理后台',
                        onClick: () => navigate('/admin'),
                      },
                      { type: 'divider' as const },
                    ]
                  : []),
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
                <span className="velora-user-chip-role">{isAdmin ? '管理员' : '普通成员'}</span>
              </span>
            </button>
          </Dropdown>
        </div>
      </Layout.Header>

      <Layout.Content className="velora-main-content velora-home-main">
        <div className="velora-home-page-content">{children}</div>
      </Layout.Content>
    </Layout>
  )
}
