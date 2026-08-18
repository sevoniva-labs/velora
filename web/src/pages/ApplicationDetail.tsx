import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { App as AntdApp, Badge, Button, Card, Descriptions, Skeleton, Tag, Typography } from 'antd'
import { ArrowLeftOutlined, HeartFilled, HeartOutlined, RocketOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { addFavorite, getApplication, launchApplication, queryKeys, removeFavorite } from '../api/api'
import { AppIcon } from '../components/AppCard'
import QueryErrorState from '../components/QueryErrorState'
import { HEALTH_COLOR, HEALTH_LABEL, SSO_TYPE_COLOR, SSO_TYPE_LABEL } from '../labels'
import { formatDateTime } from '../utils/format'
import { usePageTitle } from '../hooks/usePageTitle'

export default function ApplicationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [launching, setLaunching] = useState(false)
  // 收藏状态：初始取服务端 isFavorite，点击后乐观更新。
  const [favorited, setFavorited] = useState<boolean | null>(null)

  const { data: app, isLoading, isError, refetch } = useQuery({
    queryKey: queryKeys.application(id!),
    queryFn: () => getApplication(id!),
    enabled: !!id,
  })
  const isFavorite = favorited ?? app?.isFavorite ?? false
  usePageTitle(app?.name ? `应用详情 · ${app.name}` : '应用详情')

  const favMutation = useMutation({
    mutationFn: () => (isFavorite ? removeFavorite(id!) : addFavorite(id!)),
    onMutate: () => setFavorited((v) => !(v ?? app?.isFavorite ?? false)),
    onSuccess: () => {
      message.success(isFavorite ? '已取消收藏' : '已收藏')
      setFavorited(null)
      void queryClient.invalidateQueries({ queryKey: ['applications'] })
      void queryClient.invalidateQueries({ queryKey: ['favorites'] })
      void queryClient.invalidateQueries({ queryKey: queryKeys.application(id!) })
    },
    onError: (err) => {
      setFavorited(null)
      message.error(err instanceof Error ? err.message : '操作失败')
    },
  })

  const handleLaunch = async () => {
    if (!id) return
    setLaunching(true)
    try {
      const result = await launchApplication(id)
      if (result.url) {
        window.open(result.url, result.target === '_self' ? '_self' : '_blank', 'noopener,noreferrer')
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '启动失败')
    } finally {
      setLaunching(false)
    }
  }

  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 8 }} />
  }
  if (isError) {
    return <QueryErrorState refetch={refetch} description="应用数据加载失败，请重试。" />
  }
  if (!app) {
    return (
      <Card>
        <Typography.Text type="secondary">应用不存在或无权访问</Typography.Text>
      </Card>
    )
  }

  return (
    <div style={{ maxWidth: 960, margin: '0 auto' }}>
      <div className="velora-page-head" style={{ marginTop: 0 }}>
        <div>
          <Typography.Title level={3} className="velora-page-head-title">
            {app.name}
          </Typography.Title>
          <Typography.Paragraph className="velora-page-head-desc">
            {app.description || '暂无描述'}
          </Typography.Paragraph>
        </div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)}>
            返回
          </Button>
          <Button
            type="primary"
            icon={<RocketOutlined />}
            loading={launching}
            disabled={app.status !== 'ENABLED'}
            onClick={() => void handleLaunch()}
          >
            启动应用
          </Button>
        </div>
      </div>

      <Card className="velora-detail-section" style={{ marginTop: 0 }}>
        <div style={{ display: 'flex', gap: 16, alignItems: 'center', marginBottom: 20 }}>
          <AppIcon app={app} size={48} />
          <div style={{ minWidth: 0 }}>
            <Typography.Text strong style={{ fontSize: 15 }}>
              {app.name}
            </Typography.Text>
            <div style={{ marginTop: 6, display: 'flex', flexWrap: 'wrap', gap: 6, alignItems: 'center' }}>
              <Tag color={SSO_TYPE_COLOR[app.ssoType]}>{SSO_TYPE_LABEL[app.ssoType]}</Tag>
              {app.category && <Tag>{app.category.name}</Tag>}
              {(app.tags ?? []).map((t) => (
                <Tag key={t.id}>{t.name}</Tag>
              ))}
              {app.healthCheckEnabled && (
                <Badge
                  status={(HEALTH_COLOR[app.healthStatus ?? 'UNKNOWN'] as 'success' | 'error' | 'default') ?? 'default'}
                  text={`健康：${HEALTH_LABEL[app.healthStatus ?? 'UNKNOWN']}`}
                />
              )}
            </div>
          </div>
        </div>

        <Descriptions column={{ xs: 1, sm: 2 }} size="middle">
          <Descriptions.Item label="应用编码">{app.code}</Descriptions.Item>
          <Descriptions.Item label="分类">{app.category?.name ?? '-'}</Descriptions.Item>
          <Descriptions.Item label="接入类型">{SSO_TYPE_LABEL[app.ssoType]}</Descriptions.Item>
          <Descriptions.Item label="负责人">{app.owner || '-'}</Descriptions.Item>
          <Descriptions.Item label="所属部门">{app.department || '-'}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={app.status === 'ENABLED' ? 'success' : 'default'}>
              {app.status === 'ENABLED' ? '启用' : '停用'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="更新时间">{formatDateTime(app.updatedAt)}</Descriptions.Item>
          <Descriptions.Item label="收藏">
            <Button
              type={isFavorite ? 'default' : 'primary'}
              ghost={isFavorite}
              size="small"
              icon={isFavorite ? <HeartFilled style={{ color: '#FA541C' }} /> : <HeartOutlined />}
              loading={favMutation.isPending}
              onClick={() => favMutation.mutate()}
            >
              {isFavorite ? '已收藏' : '收藏'}
            </Button>
          </Descriptions.Item>
        </Descriptions>
      </Card>
    </div>
  )
}
