import { useQuery } from '@tanstack/react-query'
import { Empty, Skeleton, Typography } from 'antd'
import { HeartOutlined } from '@ant-design/icons'
import { listApplications, queryKeys } from '../api/api'
import { AppCard } from '../components/AppCard'
import QueryErrorState from '../components/QueryErrorState'
import { usePageTitle } from '../hooks/usePageTitle'

/** 我的收藏：刷新后仍存在（数据落库，服务端持久化）。 */
export default function Favorites() {
  usePageTitle('我的收藏')

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: queryKeys.favorites,
    queryFn: () => listApplications({ favorites: true, pageSize: 100 }),
  })

  return (
    <div>
      <div className="velora-page-head">
        <div>
          <Typography.Title level={3} className="velora-page-head-title">
            <HeartOutlined style={{ color: '#FA541C' }} /> 我的收藏
          </Typography.Title>
          <Typography.Paragraph className="velora-page-head-desc">
            已收藏 {data?.total ?? 0} 个应用，随时一键直达。
          </Typography.Paragraph>
        </div>
      </div>

      {isLoading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : isError ? (
        <QueryErrorState refetch={refetch} />
      ) : data && data.items.length > 0 ? (
        <div className="velora-app-grid" style={{ marginTop: 16 }}>
          {data.items.map((app) => (
            <div key={app.id} className="velora-app-grid-item">
              <AppCard app={app} />
            </div>
          ))}
        </div>
      ) : (
        <Empty
          description="还没有收藏任何应用，在应用中心点击 ❤ 收藏"
          style={{ background: 'var(--velora-bg-container)', borderRadius: 12, padding: '64px 0' }}
        />
      )}
    </div>
  )
}
