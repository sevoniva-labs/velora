function envText(value: unknown, fallback = ''): string {
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

/**
 * 门户展示配置在 Web 构建时注入。它不是运行时管理 API；变更需要经过
 * 版本化构建与发布，避免把浏览器本地值或假接口当成生产配置事实源。
 */
export const portalConfig = Object.freeze({
  name: envText(import.meta.env.VITE_PORTAL_NAME, 'Velora'),
  welcome: envText(import.meta.env.VITE_PORTAL_WELCOME, '企业应用门户'),
  footer: envText(import.meta.env.VITE_PORTAL_FOOTER),
  announcement: envText(import.meta.env.VITE_PORTAL_ANNOUNCEMENT),
  uiScale: envText(import.meta.env.VITE_UI_SCALE, '1'),
})
