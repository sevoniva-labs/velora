// 应用卡片：紧凑横式 —— 图标 + 名称 + 一行描述，点击整卡直达启动。
// 收藏爱心常驻右上角；健康状态以小圆点呈现（仅开启健康检查的应用）。
import { useState } from 'react'
import { App as AntdApp, Badge, Button, Tooltip } from 'antd'
import { HeartFilled, HeartOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { addFavorite, launchApplication, queryKeys, removeFavorite } from '../api/api'
import type { Application } from '../types'
import { HEALTH_COLOR, HEALTH_LABEL } from '../labels'

/** 蓝族渐变图标变体：按应用 id 稳定映射到 6 种蓝色系渐变，增加视觉层次 */
function iconVariant(id: number | string): string {
  const s = String(id)
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) >>> 0
  return `velora-app-icon--v${h % 6}`
}

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
    <span className={`velora-app-icon ${iconVariant(app.id)} ${className ?? ''}`} style={style} aria-hidden="true">
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
      aria-label={`${app.name}，点击启动`}
      onClick={() => launchMutation.mutate()}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          launchMutation.mutate()
        }
      }}
    >
      <AppIcon app={app} size={44} />
      <div className="velora-app-card-main">
        <div className="velora-app-card-name">
          {app.name}
          {app.createdAt && dayjs().diff(dayjs(app.createdAt), 'day') <= 7 && (
            <span className="velora-app-new velora-app-new--inline">新</span>
          )}
        </div>
        <div className="velora-app-card-desc">{app.description || app.category?.name || '点击直达应用'}</div>
        {(app.category || (app.tags ?? []).length > 0) && (
          <div className="velora-app-card-meta">
            {app.category && <span className="velora-app-card-cat">{app.category.name}</span>}
            {(app.tags ?? []).slice(0, 2).map((t) => (
              <span key={t.id} className="velora-app-card-tag">
                {t.name}
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="velora-app-card-side">
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
  )
}

export { AppIcon }
