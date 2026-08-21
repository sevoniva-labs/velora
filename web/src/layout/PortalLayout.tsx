// 普通用户门户外壳：深海军蓝顶栏（品牌 + 导航 + 搜索 + 用户）+ 居中内容区。
import type { ReactNode } from 'react'
import { App as AntdApp, Avatar, Dropdown, Layout } from 'antd'
import { LogoutOutlined, SettingOutlined, UserOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useState, type KeyboardEvent } from 'react'
import { useMe } from '../auth/useMe'
import { canAccessAdmin } from '../auth/permissions'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { logout, getPortalSettings, queryKeys } from '../api/api'

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

  // 门户展示配置：名称 / 欢迎语（未配置时用默认值）。
  const { data: settings } = useQuery({ queryKey: queryKeys.portalSettings, queryFn: getPortalSettings })
  const valueOf = (key: string) => settings?.find((s) => s.key === key)?.value ?? ''
  const portalName = valueOf('portal_name') || 'Velora'
  const portalWelcome = valueOf('portal_welcome') || '企业应用门户'

  const isAdmin = canAccessAdmin(me.data?.permissions, me.data?.roles)
  const displayName = me.data?.displayName || me.data?.username || '用户'
  const activeKey = NAV_ITEMS.find((i) => location.pathname.startsWith(i.path))?.key ?? 'home'

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

  // 顶栏搜索：yidian 风格玻璃搜索条，回车跳转应用中心并携带关键词。
  const [keyword, setKeyword] = useState('')
  const onHeaderSearch = (value: string) => {
    const q = value.trim()
    navigate(q ? `/applications?keyword=${encodeURIComponent(q)}` : '/applications')
  }

  return (
    <Layout className="velora-layout">
      <a className="velora-skip-link" href="#main">跳到主内容</a>
      <Layout.Header className="velora-header">
        <div className="velora-header-brand">
          <Link className="velora-brand" to="/home" aria-label="首页">
            <span className="velora-brand-mark" aria-hidden="true">
              <img src="/sevoniva-mark.svg" alt="" width={19} height={19} style={{ display: 'block' }} />
            </span>
            <span className="velora-brand-text">
              <span className="velora-brand-name">{portalName}</span>
              <span className="velora-brand-sub">{portalWelcome}</span>
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
          <div className="velora-header-search" role="search">
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="11" cy="11" r="7" />
              <path d="M21 21l-4.3-4.3" />
            </svg>
            <input
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => {
                if (e.key === 'Enter') {
                  onHeaderSearch(keyword)
                }
              }}
              placeholder="搜索应用、资讯、文档、知识库等"
              aria-label="搜索应用"
            />
          </div>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'user-center',
                  icon: <UserOutlined />,
                  label: '用户中心',
                  onClick: () => navigate('/user-center'),
                },
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

      <Layout.Content id="main" className="velora-main-content velora-home-main">
        <div className="velora-home-page-content">{children}</div>
      </Layout.Content>
    </Layout>
  )
}
