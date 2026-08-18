import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { App as AntdApp, Empty, Skeleton } from 'antd'
import {
  AppstoreOutlined,
  ArrowRightOutlined,
  FireOutlined,
  FolderOpenOutlined,
  PlusOutlined,
  SoundOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { ProCard } from '@ant-design/pro-components'
import { launchApplication, listApplications, listCategories, listPopular, listRecent, getPortalSettings, queryKeys } from '../api/api'
import { AppIcon } from '../components/AppCard'
import QueryErrorState from '../components/QueryErrorState'
import { formatRelativeTime } from '../utils/format'
import { usePageTitle } from '../hooks/usePageTitle'
import type { Application } from '../types'

/** 应用图标（分类占位）：按 code 稳定映射到一套线性图标 */
const CATEGORY_ICONS = [AppstoreOutlined, FolderOpenOutlined, FireOutlined, SoundOutlined]

function categoryIcon(code: string) {
  let h = 0
  for (let i = 0; i < code.length; i++) h = (h * 31 + code.charCodeAt(i)) >>> 0
  const Icon = CATEGORY_ICONS[h % CATEGORY_ICONS.length]
  return <Icon />
}

type Section = 'featured' | 'favorites' | 'all'

export default function Home() {
  const navigate = useNavigate()
  const { message } = AntdApp.useApp()

  // SSO 门户模型：点击应用 = 直接启动（OIDC 应用跳转 Casdoor 登录，URL 应用直接打开）。
  const launchApp = async (appId: number | string) => {
    try {
      const result = await launchApplication(appId)
      if (result.url) {
        window.open(result.url, result.target === '_self' ? '_self' : '_blank', 'noopener,noreferrer')
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '启动失败')
    }
  }
  // 我的应用 tab 内容：宫格应用（高度固定两行，超出滚动）
  const renderMyApps = () => {
    if (loadingApps) return <Skeleton active paragraph={{ rows: 3 }} />
    if (errorApps) return <QueryErrorState compact refetch={refetchApps} />
    if (myApps && myApps.items.length > 0) {
      return (
        <div className="velora-app-tile-grid">
          {myApps.items.map((app) => (
            <div
              key={app.id}
              className="velora-app-tile"
              role="button"
              tabIndex={0}
              onClick={() => launchApp(app.id)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  launchApp(app.id)
                }
              }}
            >
              <AppIcon app={app} size={38} />
              <span className="velora-app-tile-name">{app.name}</span>
            </div>
          ))}
          <div
            className="velora-app-tile velora-app-tile-add"
            role="button"
            tabIndex={0}
            onClick={() => navigate('/applications')}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                navigate('/applications')
              }
            }}
          >
            <PlusOutlined /> 添加应用
          </div>
        </div>
      )
    }
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={section === 'favorites' ? '还没有收藏应用' : '暂无可用应用'}
        style={{ padding: '28px 0' }}
      />
    )
  }

  // 公告：优先取门户设置中的 announcement（多行以 | 分隔），否则默认模板。
  const { data: settings } = useQuery({ queryKey: queryKeys.portalSettings, queryFn: getPortalSettings })
  const noticeText = settings?.find((s) => s.key === 'announcement')?.value
  const notices = noticeText
    ? noticeText.split('|').map((s) => s.trim()).filter(Boolean)
    : ['欢迎使用 Velora 企业应用门户：统一身份认证（Casdoor）已就绪，一次登录即可访问全部授权应用。', '系统公告：请妥善保管企业账号，勿在公共设备上勾选“记住登录”。']

  usePageTitle('工作台')

  const [section, setSection] = useState<Section>('featured')

  const appsQuery = {
    featured: { featured: true, pageSize: 14 },
    favorites: { favorites: true, pageSize: 14 },
    all: { pageSize: 14 },
  }[section]

  const { data: myApps, isLoading: loadingApps, isError: errorApps, refetch: refetchApps } = useQuery({
    queryKey: queryKeys.applications(appsQuery),
    queryFn: () => listApplications(appsQuery),
  })

  const { data: recent, isLoading: loadingRecent, isError: errorRecent, refetch: refetchRecent } = useQuery({
    queryKey: queryKeys.recent,
    queryFn: () => listRecent(6),
  })

  const { data: categories } = useQuery({ queryKey: queryKeys.categories, queryFn: listCategories })

  const { data: popular, isLoading: loadingPopular, isError: errorPopular, refetch: refetchPopular } = useQuery({
    queryKey: queryKeys.popular,
    queryFn: () => listPopular(6),
  })

  const { data: favApps, isLoading: loadingFav, isError: errorFav, refetch: refetchFav } = useQuery({
    queryKey: queryKeys.favorites,
    queryFn: () => listApplications({ favorites: true, pageSize: 100 }),
  })

  return (
    <div>
      {/* 通知公告条（玻璃跑马灯） */}
      <div className="velora-panel velora-notice">
        <span className="velora-notice-label">
          <SoundOutlined /> 通知公告
        </span>
        <span className="velora-notice-sep" />
        <div className="velora-notice-marquee">
          <span>
            {notices.join('　　　　')}
            {'　　　　'}
            {notices.join('　　　　')}
          </span>
        </div>
        <span className="velora-notice-more">全部 ›</span>
      </div>

      {/* 我的应用（ProCard tabs：精选 / 收藏 / 全部） */}
      <ProCard
        className="velora-panel velora-myapps"
        title="我的应用"
        extra={
          <a onClick={() => navigate('/applications')}>
            应用中心 <ArrowRightOutlined />
          </a>
        }
        tabs={{
          size: 'small',
          activeKey: section,
          onChange: (key) => setSection(key as Section),
          items: [
            { key: 'featured', label: '精选', children: renderMyApps() },
            { key: 'favorites', label: '收藏', children: renderMyApps() },
            { key: 'all', label: '全部', children: renderMyApps() },
          ],
        }}
      />

      {/* 33 / 67 双栏 */}
      <div className="velora-workbench">
        {/* 最近使用 */}
        <section className="velora-panel">
          <div className="velora-panel-head">
            <h2 className="velora-panel-title">最近使用</h2>
            <div className="velora-panel-more">
              <a onClick={() => navigate('/applications')}>
                全部应用 <ArrowRightOutlined />
              </a>
            </div>
          </div>
          {loadingRecent ? (
            <Skeleton active paragraph={{ rows: 4 }} style={{ padding: '4px 18px 16px' }} />
          ) : errorRecent ? (
            <QueryErrorState compact refetch={refetchRecent} />
          ) : recent && recent.length > 0 ? (
            <div className="velora-recent-list">
              {recent.map((item) => (
                <div
                  key={item.application.id}
                  className="velora-recent-item"
                  role="button"
                  tabIndex={0}
                  onClick={() => launchApp(item.application.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      launchApp(item.application.id)
                    }
                  }}
                >
                  <AppIcon app={item.application} size={32} />
                  <div className="velora-recent-item-main">
                    <div className="velora-recent-item-name">{item.application.name}</div>
                    <div className="velora-recent-item-meta">
                      {formatRelativeTime(item.lastVisitedAt)} · 使用 {item.visitCount} 次
                    </div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无使用记录" style={{ padding: '28px 18px' }} />
          )}
        </section>

        {/* 应用分类 */}
        <section className="velora-panel">
          <div className="velora-panel-head">
            <h2 className="velora-panel-title">应用分类</h2>
            <div className="velora-panel-more">
              <a onClick={() => navigate('/applications')}>
                全部 <ArrowRightOutlined />
              </a>
            </div>
          </div>
          {categories && categories.length > 0 ? (
            <div className="velora-cat-grid">
              {categories.slice(0, 6).map((cat) => (
                <div
                  key={cat.id}
                  className="velora-cat-card"
                  role="button"
                  tabIndex={0}
                  onClick={() => navigate(`/applications?categoryId=${cat.id}`)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      navigate(`/applications?categoryId=${cat.id}`)
                    }
                  }}
                >
                  <span className="velora-cat-card-icon">{categoryIcon(cat.code)}</span>
                  <span className="velora-cat-card-main">
                    <span className="velora-cat-card-name">{cat.name}</span>
                    <span className="velora-cat-card-desc">{cat.description || '浏览该分类应用'}</span>
                  </span>
                </div>
              ))}
            </div>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无分类" style={{ padding: '24px 0' }} />
          )}
        </section>

        {/* 热门应用 */}
        <section className="velora-panel">
          <div className="velora-panel-head">
            <h2 className="velora-panel-title">热门应用</h2>
            <div className="velora-panel-more">
              <a onClick={() => navigate('/applications')}>
                更多 <ArrowRightOutlined />
              </a>
            </div>
          </div>
          {loadingPopular ? (
            <Skeleton active paragraph={{ rows: 3 }} style={{ padding: '4px 18px 16px' }} />
          ) : errorPopular ? (
            <QueryErrorState compact refetch={refetchPopular} />
          ) : popular && popular.length > 0 ? (
            <div className="velora-popular-row">
              {popular.map((app) => (
                <div
                  key={app.id}
                  className="velora-popular-item"
                  role="button"
                  tabIndex={0}
                  onClick={() => launchApp(app.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      launchApp(app.id)
                    }
                  }}
                >
                  <AppIcon app={app} size={30} />
                  <span className="velora-popular-name">{app.name}</span>
                </div>
              ))}
            </div>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无热门应用" style={{ padding: '24px 0' }} />
          )}
        </section>

        {/* 我的收藏 */}
        <section className="velora-panel">
          <div className="velora-panel-head">
            <h2 className="velora-panel-title">我的收藏</h2>
            <div className="velora-panel-more">
              <a onClick={() => navigate('/favorites')}>
                全部 <ArrowRightOutlined />
              </a>
            </div>
          </div>
          {loadingFav ? (
            <Skeleton active paragraph={{ rows: 3 }} style={{ padding: '4px 18px 16px' }} />
          ) : errorFav ? (
            <QueryErrorState compact refetch={refetchFav} />
          ) : favApps && favApps.items.length > 0 ? (
            <div className="velora-popular-row">
              {favApps.items.slice(0, 6).map((app: Application) => (
                <div
                  key={app.id}
                  className="velora-popular-item"
                  role="button"
                  tabIndex={0}
                  onClick={() => launchApp(app.id)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      launchApp(app.id)
                    }
                  }}
                >
                  <AppIcon app={app} size={30} />
                  <span className="velora-popular-name">{app.name}</span>
                </div>
              ))}
            </div>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无收藏，点击应用卡片上的爱心收藏" style={{ padding: '24px 0' }} />
          )}
        </section>
      </div>
    </div>
  )
}
