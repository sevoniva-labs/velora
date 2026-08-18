import { useQuery } from '@tanstack/react-query'
import { Button, Empty, Input, Skeleton } from 'antd'
import { ArrowRightOutlined, ClockCircleOutlined, StarOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { listApplications, listRecent, queryKeys } from '../api/api'
import { useMe } from '../auth/useMe'
import { AppCard } from '../components/AppCard'
import { formatRelativeTime } from '../utils/format'

export default function Home() {
  const navigate = useNavigate()
  const me = useMe()

  const { data: featured, isLoading: loadingFeatured } = useQuery({
    queryKey: queryKeys.applications({ featured: true }),
    queryFn: () => listApplications({ featured: true, pageSize: 8 }),
  })

  const { data: recent, isLoading: loadingRecent } = useQuery({
    queryKey: queryKeys.recent,
    queryFn: () => listRecent(8),
  })

  const { data: allApps } = useQuery({
    queryKey: queryKeys.applications({ pageSize: 1 }),
    queryFn: () => listApplications({ pageSize: 1 }),
  })
  const { data: favorites } = useQuery({
    queryKey: queryKeys.favorites,
    queryFn: () => listApplications({ favorites: true, pageSize: 1 }),
  })

  const greeting = (() => {
    const h = new Date().getHours()
    if (h < 6) return '夜深了'
    if (h < 12) return '早上好'
    if (h < 18) return '下午好'
    return '晚上好'
  })()

  const today = new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  }).format(new Date())

  const displayName = me.data?.displayName || me.data?.username || ''

  return (
    <div>
      {/* Hero：品牌化欢迎 + 全局搜索 + 快捷统计 */}
      <section className="velora-hero" aria-label="欢迎">
        <div className="velora-hero-inner">
          <p className="velora-hero-eyebrow">Enterprise Application Portal · {today}</p>
          <h1 className="velora-hero-title">
            {greeting}
            {displayName ? <>，<span className="velora-hero-name">{displayName}</span></> : ''}
          </h1>
          <p className="velora-hero-desc">
            一个门户，一次登录，访问您有权使用的全部企业应用。搜索、收藏、直达，一步到位。
          </p>

          <div className="velora-hero-search">
            <Input.Search
              size="large"
              allowClear
              placeholder="搜索应用：名称 / 编码 / 描述 / 标签"
              enterButton="搜索"
              onSearch={(value) => {
                const q = value.trim()
                navigate(q ? `/applications?keyword=${encodeURIComponent(q)}` : '/applications')
              }}
            />
          </div>

          <div className="velora-hero-stats">
            <div className="velora-hero-stat">
              <span className="velora-hero-stat-value">{allApps?.total ?? '—'}</span>
              <span className="velora-hero-stat-label">可用应用</span>
            </div>
            <div className="velora-hero-stat">
              <span className="velora-hero-stat-value">{favorites?.total ?? '—'}</span>
              <span className="velora-hero-stat-label">我的收藏</span>
            </div>
            <div className="velora-hero-stat">
              <span className="velora-hero-stat-value">{recent?.length ?? '—'}</span>
              <span className="velora-hero-stat-label">最近使用</span>
            </div>
          </div>
        </div>
      </section>

      {/* 最近使用 */}
      <section className="velora-section-panel">
        <div className="velora-section-head">
          <h2 className="velora-section-title">
            <ClockCircleOutlined />
            最近使用
          </h2>
          <Button type="link" className="velora-section-more" onClick={() => navigate('/applications')}>
            全部应用 <ArrowRightOutlined />
          </Button>
        </div>
        {loadingRecent ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : recent && recent.length > 0 ? (
          <div className="velora-app-grid">
            {recent.map((item) => (
              <div key={item.application.id} className="velora-app-grid-item">
                <AppCard app={item.application} />
                <div className="velora-recent-time">最近使用：{formatRelativeTime(item.lastVisitedAt)}</div>
              </div>
            ))}
          </div>
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="暂无最近使用记录，从应用中心开始探索吧"
            style={{ background: 'var(--velora-bg-container)', borderRadius: 16, padding: '36px 0' }}
          />
        )}
      </section>

      {/* 精选应用 */}
      <section className="velora-section-panel">
        <div className="velora-section-head">
          <h2 className="velora-section-title">
            <StarOutlined />
            精选应用
          </h2>
          <Button type="link" className="velora-section-more" onClick={() => navigate('/applications')}>
            应用中心 <ArrowRightOutlined />
          </Button>
        </div>
        {loadingFeatured ? (
          <Skeleton active paragraph={{ rows: 3 }} />
        ) : featured && featured.items.length > 0 ? (
          <div className="velora-app-grid">
            {featured.items.map((app) => (
              <div key={app.id} className="velora-app-grid-item">
                <AppCard app={app} />
              </div>
            ))}
          </div>
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="暂无精选应用"
            style={{ background: 'var(--velora-bg-container)', borderRadius: 16, padding: '36px 0' }}
          />
        )}
      </section>
    </div>
  )
}
