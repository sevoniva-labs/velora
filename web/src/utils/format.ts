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

/**
 * Validate a user-visible external navigation target. The backend remains the
 * authority for application registration; this is a browser-side defense for
 * Todo/legacy responses and rejects credentials, non-HTTPS schemes, and
 * loopback/private/link-local destinations.
 */
export function isSafeExternalHttpsUrl(value: string): boolean {
  if (!/^https:\/\/[^/\s]+/i.test(value)) return false
  try {
    const u = new URL(value)
    const host = u.hostname.replace(/^\[|\]$/g, '').toLowerCase()
    if (!host || u.username || u.password || [...value].some((char) => char.charCodeAt(0) <= 31 || char.charCodeAt(0) === 127)) return false
    if (host === 'localhost' || host.endsWith('.localhost') || host.endsWith('.local')) return false
    if (isPrivateIPv4(host) || isPrivateIPv6(host)) return false
    return true
  } catch {
    return false
  }
}

function isPrivateIPv4(host: string): boolean {
  const octets = host.split('.').map((part) => Number(part))
  if (octets.length !== 4 || octets.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return false
  const [a, b] = octets
  return a === 0 || a === 10 || a === 127 || (a === 100 && b >= 64 && b <= 127) || (a === 169 && b === 254) || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168)
}

function isPrivateIPv6(host: string): boolean {
  const normalized = host.toLowerCase()
  return normalized === '::1' || normalized === '::' || normalized.startsWith('fc') || normalized.startsWith('fd') || normalized.startsWith('fe8') || normalized.startsWith('fe9') || normalized.startsWith('fea') || normalized.startsWith('feb')
}
