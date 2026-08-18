import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { App as AntdApp, Badge, Button, Card, Descriptions, Skeleton, Tag, Typography } from 'antd'
import { ArrowLeftOutlined, HeartFilled, HeartOutlined, RocketOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { getApplication, launchApplication, queryKeys } from '../api/api'
import { AppIcon } from '../components/AppCard'
import { HEALTH_COLOR, HEALTH_LABEL, SSO_TYPE_COLOR, SSO_TYPE_LABEL } from '../labels'
import { formatDateTime } from '../utils/format'

export default function ApplicationDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { message } = AntdApp.useApp()
  const [launching, setLaunching] = useState(false)

  const { data: app, isLoading } = useQuery({
    queryKey: queryKeys.application(id!),
    queryFn: () => getApplication(id!),
    enabled: !!id,
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
  if (!app) {
    return (
      <Card>
        <Typography.Text type="secondary">应用不存在或无权访问</Typography.Text>
      </Card>
    )
  }

  return (
    <div style={{ maxWidth: 960, margin: '0 auto', paddingTop: 12 }}>
      <Button
        type="text"
        icon={<ArrowLeftOutlined />}
        onClick={() => navigate(-1)}
        style={{ marginBottom: 14, color: 'var(--velora-text)' }}
      >
        返回
      </Button>

      {/* 深色品牌 Hero */}
      <section className="velora-detail-hero">
        <AppIcon app={app} size={64} />
        <div className="velora-detail-hero-main">
          <Typography.Title level={2} className="velora-detail-hero-title">
            {app.name}
          </Typography.Title>
          <Typography.Paragraph className="velora-detail-hero-desc">
            {app.description || '暂无描述'}
          </Typography.Paragraph>
          <div className="velora-detail-hero-meta">
            <Tag color={SSO_TYPE_COLOR[app.ssoType]}>{SSO_TYPE_LABEL[app.ssoType]}</Tag>
            {app.isFeatured && <Tag color="gold">精选</Tag>}
            {app.category && <Tag>{app.category.name}</Tag>}
            {app.tags.map((t) => (
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
        <div className="velora-detail-hero-action">
          <Button
            type="primary"
            size="large"
            icon={<RocketOutlined />}
            loading={launching}
            disabled={app.status !== 'ENABLED'}
            onClick={() => void handleLaunch()}
          >
            启动应用
          </Button>
        </div>
      </section>

      <Card title="应用信息" className="velora-detail-section">
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
            {app.isFavorite ? (
              <>
                <HeartFilled style={{ color: '#E5484D' }} /> 已收藏
              </>
            ) : (
              <>
                <HeartOutlined /> 未收藏
              </>
            )}
          </Descriptions.Item>
        </Descriptions>
      </Card>
    </div>
  )
}
