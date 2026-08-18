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

  const greeting = (() => {
    const h = new Date().getHours()
    if (h < 6) return '夜深了'
    if (h < 12) return '早上好'
    if (h < 18) return '下午好'
    return '晚上好'
  })()

  const displayName = me.data?.displayName || me.data?.username || ''

  return (
    <div>
      {/* 页头：问候 + 搜索（紧凑，不做大横幅） */}
      <div className="velora-page-head">
        <div>
          <h1 className="velora-page-head-title">
            {greeting}
            {displayName ? <>，{displayName}</> : ''}
          </h1>
          <p className="velora-page-head-desc">企业应用统一入口，快速找到并使用您有权访问的应用。</p>
        </div>
        <Input.Search
          allowClear
          placeholder="搜索应用"
          style={{ width: 320 }}
          onSearch={(value) => {
            const q = value.trim()
            navigate(q ? `/applications?keyword=${encodeURIComponent(q)}` : '/applications')
          }}
        />
      </div>

      {/* 最近使用 */}
      <section className="velora-section-panel">
        <div className="velora-section-head">
          <h2 className="velora-section-title">
            <ClockCircleOutlined />
            最近使用
          </h2>
          <Button type="link" size="small" className="velora-section-more" onClick={() => navigate('/applications')}>
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
            style={{ background: 'var(--velora-bg-container)', borderRadius: 10, padding: '28px 0' }}
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
          <Button type="link" size="small" className="velora-section-more" onClick={() => navigate('/applications')}>
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
            style={{ background: 'var(--velora-bg-container)', borderRadius: 10, padding: '28px 0' }}
          />
        )}
      </section>
    </div>
  )
}
