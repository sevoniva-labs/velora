// 应用卡片：紧凑横式 —— 图标 + 名称 + 一行描述，点击整卡直达启动。
// 收藏爱心常驻右上角；健康状态以小圆点呈现（仅开启健康检查的应用）。
import { useState } from 'react'
import { App as AntdApp, Badge, Button, Tooltip } from 'antd'
import { HeartFilled, HeartOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { addFavorite, launchApplication, queryKeys, removeFavorite } from '../api/api'
import type { Application } from '../types'
import { HEALTH_COLOR, HEALTH_LABEL } from '../labels'
import { isSafeExternalHttpsUrl } from '../utils/format'

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
  // 收藏状态以服务端 isFavorite 为唯一事实来源；override 仅承担点击后的乐观显示，
  // 待列表失效刷新完成（服务端数据回到最新）后释放，避免本地镜像状态与服务器漂移。
  const [favOverride, setFavOverride] = useState<boolean | null>(null)
  const favorited = favOverride ?? !!app.isFavorite

  const invalidateApps = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['applications'] }),
      queryClient.invalidateQueries({ queryKey: ['favorites'] }),
      queryClient.invalidateQueries({ queryKey: ['recent'] }),
      queryClient.invalidateQueries({ queryKey: queryKeys.me }),
    ])

  const favMutation = useMutation({
    mutationFn: (next: boolean) => (next ? addFavorite(app.id) : removeFavorite(app.id)),
    onMutate: (next) => setFavOverride(next),
    onSuccess: async (_result, next) => {
      message.success(next ? '已收藏' : '已取消收藏')
      await invalidateApps()
      setFavOverride(null)
    },
    onError: (err) => {
      setFavOverride(null)
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
        if (!isSafeExternalHttpsUrl(result.url)) {
          message.error('应用地址不符合安全策略')
          return
        }
        window.open(result.url, result.target === '_self' ? '_self' : '_blank', 'noopener,noreferrer')
      }
      void invalidateApps()
    },
    onError: (err) => {
      message.error(err instanceof Error ? err.message : '启动失败')
    },
  })

  return (
    <div className="velora-app-card">
      <button type="button" className="velora-app-card-hitarea" aria-label={`${app.name}，点击启动`} onClick={() => launchMutation.mutate()}>
        <AppIcon app={app} size={44} />
        <span className="velora-app-card-main">
          <span className="velora-app-card-name">
            {app.name}
            {app.isNew && <span className="velora-app-new velora-app-new--inline">新</span>}
          </span>
          <span className="velora-app-card-desc">{app.description || app.category?.name || '点击直达应用'}</span>
          {(app.category || (app.tags ?? []).length > 0) && (
            <span className="velora-app-card-meta">
              {app.category && <span className="velora-app-card-cat">{app.category.name}</span>}
              {(app.tags ?? []).slice(0, 2).map((t) => (
                <span key={t.id} className="velora-app-card-tag">
                  {t.name}
                </span>
              ))}
            </span>
          )}
        </span>
      </button>
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
            favMutation.mutate(!favorited)
          }}
        />
      </div>
    </div>
  )
}

export { AppIcon }
