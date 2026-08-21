import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { App as AntdApp, Button, Empty, Modal, Skeleton } from 'antd'
import {
  AppstoreOutlined,
  ArrowRightOutlined,
  DatabaseOutlined,
  FireOutlined,
  FolderOpenOutlined,
  HeartOutlined,
  PlusOutlined,
  RocketOutlined,
  SoundOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { ProCard } from '@ant-design/pro-components'
import { launchApplication, listApplications, listCategories, listRecent, getPortalSettings, queryKeys } from '../api/api'
import { AppIcon } from '../components/AppCard'
import TodoCenter from '../components/TodoCenter'
import QueryErrorState from '../components/QueryErrorState'
import { formatRelativeTime } from '../utils/format'
import { usePageTitle } from '../hooks/usePageTitle'

/** 应用图标（分类占位）：按 code 稳定映射到一套线性图标 */
const CATEGORY_ICONS = [
  AppstoreOutlined,
  FolderOpenOutlined,
  FireOutlined,
  SoundOutlined,
  ToolOutlined,
  DatabaseOutlined,
  TeamOutlined,
  RocketOutlined,
]

function categoryIcon(code: string) {
  // FNV-1a + 雪崩混合：短编码也能均匀离散到 8 个图标
  let h = 2166136261
  for (let i = 0; i < code.length; i++) {
    h ^= code.charCodeAt(i)
    h = (h * 16777619) >>> 0
  }
  h ^= h >>> 16
  h = (h * 2246822507) >>> 0
  h ^= h >>> 13
  h = (h * 3266489909) >>> 0
  h ^= h >>> 16
  const Icon = CATEGORY_ICONS[(h >>> 0) % CATEGORY_ICONS.length]
  return <Icon />
}

type Section = 'favorites' | 'all'

export default function Home() {
  const navigate = useNavigate()
  const { message } = AntdApp.useApp()

  // SSO 门户模型：点击应用 = 直接启动（OIDC 应用跳转外部 IdP，URL 应用直接打开）。
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
  // 我的应用 tab 内容：宫格应用（高度固定两行，超出滚动；加载/空状态同高，切换不跳动）
  const renderMyApps = () => {
    if (loadingApps)
      return (
        <div className="velora-myapps-state">
          <Skeleton active paragraph={{ rows: 2 }} style={{ width: '100%' }} />
        </div>
      )
    if (errorApps)
      return (
        <div className="velora-myapps-state">
          <QueryErrorState compact refetch={refetchApps} />
        </div>
      )
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
              {app.isNew && <span className="velora-app-new">新</span>}
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
    // 收藏为空：引导去应用中心，而不是冷冰冰的空状态
    if (section === 'favorites') {
      return (
        <div className="velora-myapps-state">
          <div className="velora-myapps-guide">
            <HeartOutlined className="velora-myapps-guide-icon" />
            <span className="velora-myapps-guide-text">把常用应用钉在这里，一键直达</span>
            <Button size="small" type="primary" onClick={() => navigate('/applications')}>
              去应用中心
            </Button>
          </div>
        </div>
      )
    }
    return (
      <div className="velora-myapps-state">
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可用应用" />
      </div>
    )
  }

  // 公告：取门户设置中的 announcement（多条以 | 分隔），未配置则不展示公告条。
  const { data: settings } = useQuery({ queryKey: queryKeys.portalSettings, queryFn: getPortalSettings })
  const noticeText = settings?.find((s) => s.key === 'announcement')?.value
  const notices = noticeText ? noticeText.split('|').map((s) => s.trim()).filter(Boolean) : []
  // 全部公告弹窗
  const [noticeOpen, setNoticeOpen] = useState(false)

  usePageTitle('工作台')

  const [section, setSection] = useState<Section>('favorites')
  // 智能默认：首次数据到达后，无收藏则默认"全部"；用户手动切换后不再干预
  const sectionTouched = useRef(false)
  const switchSection = (key: Section) => {
    sectionTouched.current = true
    setSection(key)
  }

  const appsQuery = {
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

  const { data: favApps } = useQuery({
    queryKey: queryKeys.favorites,
    queryFn: () => listApplications({ favorites: true, pageSize: 100 }),
  })

  // 全量列表（一次拉取）：分段控件计数 + 分类计数共用
  const { data: allAppsList } = useQuery({
    queryKey: queryKeys.applications({ pageSize: 500 }),
    queryFn: () => listApplications({ pageSize: 500 }),
  })

  const favTotal = favApps?.total ?? 0
  const allTotal = allAppsList?.total ?? 0

  useEffect(() => {
    if (!sectionTouched.current && favApps) {
      setSection((favApps.total ?? 0) > 0 ? 'favorites' : 'all')
    }
  }, [favApps])

  // 分类计数（前端统计）；有计数的分类优先展示，全未分类时回退展示全部
  const catCounts = useMemo(() => {
    const m = new Map<string | number, number>()
    allAppsList?.items.forEach((a) => {
      if (a.categoryId != null) m.set(a.categoryId, (m.get(a.categoryId) ?? 0) + 1)
    })
    return m
  }, [allAppsList])
  const visibleCategories = useMemo(() => {
    const all = categories ?? []
    if (!allAppsList) return all.slice(0, 8) // 计数未加载完，先全量展示避免闪空
    const withApps = all.filter((c) => (catCounts.get(c.id) ?? 0) > 0).slice(0, 8)
    return withApps.length > 0 ? withApps : all.slice(0, 8)
  }, [categories, allAppsList, catCounts])

  return (
    <div>
      {/* 通知公告条（玻璃跑马灯），有公告时才渲染 */}
      {notices.length > 0 ? (
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
          <a className="velora-notice-more" onClick={() => setNoticeOpen(true)}>
            全部 ›
          </a>
        </div>
      ) : null}

      {/* 全部公告弹窗 */}
      <Modal title="通知公告" open={noticeOpen} onCancel={() => setNoticeOpen(false)} footer={null} width={520}>
        <div className="velora-notice-list">
          {notices.map((n) => (
            <div key={n} className="velora-notice-item">
              <SoundOutlined className="velora-notice-item-icon" />
              <span>{n}</span>
            </div>
          ))}
        </div>
      </Modal>

      {/* 我的应用（胶囊分段控件：收藏 / 全部，带计数，智能默认） */}
      <ProCard
        className="velora-panel velora-myapps"
        title={
          <div className="velora-myapps-headline">
            <span className="velora-myapps-title">我的应用</span>
            <div className="velora-segment" role="tablist" aria-label="应用筛选">
              {(
                [
                  { key: 'favorites', label: '收藏', count: favTotal },
                  { key: 'all', label: '全部', count: allTotal },
                ] as const
              ).map((item) => (
                <button
                  key={item.key}
                  type="button"
                  role="tab"
                  aria-selected={section === item.key}
                  className={
                    section === item.key ? 'velora-segment-item is-active' : 'velora-segment-item'
                  }
                  onClick={() => switchSection(item.key)}
                >
                  {item.label}
                  <span className="velora-segment-count">{item.count}</span>
                </button>
              ))}
            </div>
          </div>
        }
        extra={
          <a className="velora-myapps-more" onClick={() => navigate('/applications')}>
            应用中心 <ArrowRightOutlined />
          </a>
        }
      >
        {renderMyApps()}
      </ProCard>

      {/* 三栏工作台：最近使用 / 待办中心 / 应用分类。 */}
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

        {/* 待办中心：多 Tab（邮件入口保留，邮件服务暂不接入）。 */}
        <TodoCenter />

        {/* 应用分类（带计数，空分类不展示） */}
        <section className="velora-panel">
          <div className="velora-panel-head">
            <h2 className="velora-panel-title">应用分类</h2>
            <div className="velora-panel-more">
              <a onClick={() => navigate('/applications')}>
                全部 <ArrowRightOutlined />
              </a>
            </div>
          </div>
          {visibleCategories.length > 0 ? (
            <div className="velora-cat-grid">
              {visibleCategories.map((cat) => (
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
                  {(catCounts.get(cat.id) ?? 0) > 0 && (
                    <span className="velora-cat-count">{catCounts.get(cat.id)}</span>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无分类" style={{ padding: '24px 0' }} />
          )}
        </section>
      </div>

    </div>
  )
}
