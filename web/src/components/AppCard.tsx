// 应用卡片：标准企业组件 —— 图标 / 名称 / 描述 / 分类标签 / 收藏 / 启动。
// 使用 AntD 标准品质：浅灰图标底、标准 Tag 与 Button，hover 仅阴影变化。
import { useState } from 'react'
import { App as AntdApp, Badge, Button, Tag, Tooltip, Typography } from 'antd'
import { HeartFilled, HeartOutlined, RocketOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { addFavorite, launchApplication, queryKeys, removeFavorite } from '../api/api'
import type { Application } from '../types'
import { APP_STATUS_LABEL, HEALTH_COLOR, HEALTH_LABEL } from '../labels'

function AppIcon({ app, size = 40, className }: { app: Application; size?: number; className?: string }) {
  const icon = app.icon?.trim()
  // 图标 URL 或 emoji / 文本。
  const isUrl = icon ? /^https?:\/\//i.test(icon) : false
  const isEmoji = icon ? /[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u.test(icon) : false
  const style = { width: size, height: size, fontSize: size * 0.48 }
  if (isUrl) {
    return (
      <span className={`velora-app-icon ${className ?? ''}`} style={{ ...style, overflow: 'hidden' }}>
        <img
          src={icon}
          alt={app.name}
          width={size * 0.6}
          height={size * 0.6}
          style={{ objectFit: 'contain', display: 'block' }}
          loading="lazy"
        />
      </span>
    )
  }
  return (
    <span className={`velora-app-icon ${className ?? ''}`} style={style} aria-hidden="true">
      {isEmoji ? icon : app.name.slice(0, 1)}
    </span>
  )
}

export interface AppCardProps {
  app: Application
  onLaunch?: (app: Application) => void
}

export function AppCard({ app, onLaunch }: AppCardProps) {
  const { message } = AntdApp.useApp()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [favorited, setFavorited] = useState(!!app.isFavorite)

  const invalidateApps = () => {
    void queryClient.invalidateQueries({ queryKey: ['applications'] })
    void queryClient.invalidateQueries({ queryKey: ['favorites'] })
    void queryClient.invalidateQueries({ queryKey: ['recent'] })
    void queryClient.invalidateQueries({ queryKey: queryKeys.me })
  }

  const favMutation = useMutation({
    mutationFn: () => (favorited ? removeFavorite(app.id) : addFavorite(app.id)),
    onMutate: () => setFavorited((v) => !v),
    onSuccess: () => {
      message.success(favorited ? '已取消收藏' : '已收藏')
      invalidateApps()
    },
    onError: (err) => {
      setFavorited((v) => !v)
      message.error(err instanceof Error ? err.message : '操作失败')
    },
  })

  const launchMutation = useMutation({
    mutationFn: () => launchApplication(app.id),
    onSuccess: (result) => {
      if (onLaunch) {
        onLaunch(app)
        return
      }
      if (result.url) {
        window.open(result.url, result.target === '_self' ? '_self' : '_blank', 'noopener,noreferrer')
      }
      invalidateApps()
    },
    onError: (err) => {
      message.error(err instanceof Error ? err.message : '启动失败')
    },
  })

  return (
    <div
      className="velora-app-card"
      role="button"
      tabIndex={0}
      aria-label={`${app.name}，点击查看详情`}
      onClick={() => navigate(`/applications/${app.id}`)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          navigate(`/applications/${app.id}`)
        }
      }}
    >
      <div className="velora-app-card-head">
        <AppIcon app={app} />
        <div className="velora-app-card-actions">
          {app.healthCheckEnabled && (
            <Tooltip title={`健康状态：${HEALTH_LABEL[app.healthStatus ?? 'UNKNOWN']}`}>
              <Badge
                status={(HEALTH_COLOR[app.healthStatus ?? 'UNKNOWN'] as 'success' | 'error' | 'default') ?? 'default'}
                data-testid="health-badge"
              />
            </Tooltip>
          )}
          <Button
            type="text"
            size="small"
            aria-label={favorited ? '取消收藏' : '收藏'}
            icon={favorited ? <HeartFilled style={{ color: '#FA541C' }} /> : <HeartOutlined />}
            onClick={(e) => {
              e.stopPropagation()
              favMutation.mutate()
            }}
          />
        </div>
      </div>

      <Typography.Title level={5} className="velora-app-card-title" ellipsis={{ rows: 1 }}>
        {app.name}
      </Typography.Title>
      <Typography.Paragraph className="velora-app-card-desc" ellipsis={{ rows: 2 }}>
        {app.description || '暂无描述'}
      </Typography.Paragraph>

      <div className="velora-app-card-meta">
        {app.category && <Tag className="velora-app-card-cat">{app.category.name}</Tag>}
        {(app.tags ?? []).slice(0, 3).map((t) => (
          <Tag key={t.id} className="velora-app-card-tag">
            {t.name}
          </Tag>
        ))}
      </div>

      <div className="velora-app-card-foot">
        <Tag color={app.status === 'ENABLED' ? 'success' : 'default'}>
          {APP_STATUS_LABEL[app.status]}
        </Tag>
        <Button
          type="primary"
          size="small"
          icon={<RocketOutlined />}
          loading={launchMutation.isPending}
          disabled={app.status !== 'ENABLED'}
          onClick={(e) => {
            e.stopPropagation()
            launchMutation.mutate()
          }}
        >
          启动
        </Button>
      </div>
    </div>
  )
}

export { AppIcon }
