// 界面缩放：读取构建配置中的 VITE_UI_SCALE（如 "1.1"），整体等比例缩放到根元素。
// 解决部署到不同分辨率 / 浏览器缩放的机器后，界面显示大小与本地不一致的问题。
// 效果等同浏览器 Ctrl + / -，对布局与组件统一生效。
import { useEffect } from 'react'
import { portalConfig } from '../config/portal'

const DEFAULT_SCALE = 1
const MIN_SCALE = 0.8
const MAX_SCALE = 1.4

export default function UiScale() {
  useEffect(() => {
    const raw = portalConfig.uiScale
    const parsed = raw ? Number.parseFloat(raw) : NaN
    const scale = Number.isFinite(parsed) ? Math.min(MAX_SCALE, Math.max(MIN_SCALE, parsed)) : DEFAULT_SCALE
    const root = document.documentElement
    root.style.zoom = String(scale)
    return () => {
      root.style.zoom = ''
    }
  }, [])

  return null
}
