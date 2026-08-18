import { useQuery } from '@tanstack/react-query'
import { Empty, Skeleton } from 'antd'
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
      {isLoading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : isError ? (
        <QueryErrorState refetch={refetch} />
      ) : data && data.items.length > 0 ? (
        <div className="velora-app-grid">
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
