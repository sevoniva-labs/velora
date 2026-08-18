// 通用工具函数（纯函数，便于单测）。

import dayjs from 'dayjs'

/** 相对时间展示：刚刚 / N 分钟前 / N 小时前 / N 天前 / 日期。 */
export function formatRelativeTime(value?: string): string {
  if (!value) return '-'
  const time = dayjs(value)
  if (!time.isValid()) return '-'
  const diffMin = dayjs().diff(time, 'minute')
  if (diffMin < 1) return '刚刚'
  if (diffMin < 60) return `${diffMin} 分钟前`
  const diffHour = dayjs().diff(time, 'hour')
  if (diffHour < 24) return `${diffHour} 小时前`
  const diffDay = dayjs().diff(time, 'day')
  if (diffDay < 7) return `${diffDay} 天前`
  return time.format('YYYY-MM-DD HH:mm')
}

/** 完整时间展示。 */
export function formatDateTime(value?: string): string {
  if (!value) return '-'
  const time = dayjs(value)
  return time.isValid() ? time.format('YYYY-MM-DD HH:mm:ss') : '-'
}

/** 安全裁剪：限制字符串长度，超出加省略号。 */
export function truncate(value: string, max: number): string {
  if (!value) return ''
  if (value.length <= max) return value
  return `${value.slice(0, max)}…`
}

/** 判断 URL 是否 http/https（防 javascript: 等危险 scheme，且要求存在主机名）。 */
export function isSafeHttpUrl(value: string): boolean {
  // scheme:// 后必须紧跟非空主机名（拦截 https:///path 这类无主机写法）。
  if (!/^https?:\/\/[^/\s]+/i.test(value)) return false
  try {
    const u = new URL(value)
    return u.host !== ''
  } catch {
    return false
  }
}
