// 界面缩放：读取门户设置中的 ui_scale（如 "1.1"），整体等比例缩放到根元素。
// 解决部署到不同分辨率 / 浏览器缩放的机器后，界面显示大小与本地不一致的问题。
// 效果等同浏览器 Ctrl + / -，对布局与组件统一生效。
import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getPortalSettings, queryKeys } from '../api/api'

const DEFAULT_SCALE = 1
const MIN_SCALE = 0.8
const MAX_SCALE = 1.4

export default function UiScale() {
  const { data: settings } = useQuery({
    queryKey: queryKeys.portalSettings,
    queryFn: getPortalSettings,
    // 未登录时会 401，静默忽略，登录后自动重新拉取。
    retry: false,
  })

  useEffect(() => {
    const raw = settings?.find((s) => s.key === 'ui_scale')?.value
    const parsed = raw ? Number.parseFloat(raw) : NaN
    const scale = Number.isFinite(parsed) ? Math.min(MAX_SCALE, Math.max(MIN_SCALE, parsed)) : DEFAULT_SCALE
    const root = document.documentElement
    root.style.zoom = String(scale)
    return () => {
      root.style.zoom = ''
    }
  }, [settings])

  return null
}
