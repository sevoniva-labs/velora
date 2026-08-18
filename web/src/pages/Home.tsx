import { useQuery } from '@tanstack/react-query'
import { Button, Empty, Input, Skeleton, Typography } from 'antd'
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
      {/* 欢迎区 */}
      <div style={{ padding: '28px 0 8px' }}>
        <Typography.Title level={2} style={{ marginBottom: 8, color: 'var(--velora-title)' }}>
          {greeting}，{displayName || '欢迎'}
        </Typography.Title>
        <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0, fontSize: 14 }}>
          快速找到并使用您有权访问的企业应用。
        </Typography.Paragraph>
      </div>

      {/* 全局搜索 */}
      <div style={{ margin: '24px 0 8px' }}>
        <Input.Search
          size="large"
          allowClear
          placeholder="搜索应用：名称 / 编码 / 描述 / 标签 / 关键词"
          enterButton="搜索"
          style={{ maxWidth: 640 }}
          onSearch={(value) => {
            const q = value.trim()
            navigate(q ? `/applications?keyword=${encodeURIComponent(q)}` : '/applications')
          }}
        />
      </div>

      {/* 最近使用 */}
      <section style={{ marginTop: 36 }}>
        <div className="velora-section-head">
          <Typography.Title level={4} style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
            <ClockCircleOutlined style={{ color: 'var(--velora-primary)' }} />
            最近使用
          </Typography.Title>
          <Button type="link" onClick={() => navigate('/applications')}>
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
            style={{ background: 'var(--velora-bg-container)', borderRadius: 12, padding: '32px 0' }}
          />
        )}
      </section>

      {/* 精选应用 */}
      <section style={{ marginTop: 36 }}>
        <div className="velora-section-head">
          <Typography.Title level={4} style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
            <StarOutlined style={{ color: 'var(--velora-primary)' }} />
            精选应用
          </Typography.Title>
          <Button type="link" onClick={() => navigate('/applications')}>
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
            style={{ background: 'var(--velora-bg-container)', borderRadius: 12, padding: '32px 0' }}
          />
        )}
      </section>
    </div>
  )
}
