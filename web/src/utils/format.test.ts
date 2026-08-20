import { describe, expect, it } from 'vitest'
import { formatRelativeTime, truncate, isSafeHttpUrl, isSafeExternalHttpsUrl } from './format'
import dayjs from 'dayjs'

describe('formatRelativeTime', () => {
  it('空值返回 -', () => {
    expect(formatRelativeTime(undefined)).toBe('-')
    expect(formatRelativeTime('')).toBe('-')
  })

  it('无效时间返回 -', () => {
    expect(formatRelativeTime('not-a-date')).toBe('-')
  })

  it('刚刚', () => {
    expect(formatRelativeTime(dayjs().subtract(10, 'second').toISOString())).toBe('刚刚')
  })

  it('分钟前', () => {
    expect(formatRelativeTime(dayjs().subtract(5, 'minute').toISOString())).toBe('5 分钟前')
  })

  it('小时前', () => {
    expect(formatRelativeTime(dayjs().subtract(3, 'hour').toISOString())).toBe('3 小时前')
  })

  it('天前', () => {
    expect(formatRelativeTime(dayjs().subtract(2, 'day').toISOString())).toBe('2 天前')
  })

  it('超过 7 天返回日期时间', () => {
    const v = dayjs().subtract(30, 'day').format('YYYY-MM-DD HH:mm')
    expect(formatRelativeTime(dayjs().subtract(30, 'day').toISOString())).toBe(v)
  })
})

describe('truncate', () => {
  it('短文本原样返回', () => {
    expect(truncate('hello', 10)).toBe('hello')
  })
  it('超长文本截断加省略号', () => {
    expect(truncate('hello world', 5)).toBe('hello…')
  })
  it('空值返回空串', () => {
    expect(truncate('', 3)).toBe('')
  })
})

describe('isSafeHttpUrl', () => {
  it('http/https 合法', () => {
    expect(isSafeHttpUrl('https://app.example.internal')).toBe(true)
    expect(isSafeHttpUrl('http://10.0.0.1:8080/path')).toBe(true)
  })
  it('危险 scheme 拒绝', () => {
    expect(isSafeHttpUrl('javascript:alert(1)')).toBe(false)
    expect(isSafeHttpUrl('file:///etc/passwd')).toBe(false)
    expect(isSafeHttpUrl('data:text/html,x')).toBe(false)
  })
  it('无主机拒绝', () => {
    expect(isSafeHttpUrl('https:///path')).toBe(false)
  })
  it('非法输入拒绝', () => {
    expect(isSafeHttpUrl('not a url')).toBe(false)
  })
})

describe('isSafeExternalHttpsUrl', () => {
  it('只允许无凭据的 HTTPS 公网目标', () => {
    expect(isSafeExternalHttpsUrl('https://app.example.com/path')).toBe(true)
    expect(isSafeExternalHttpsUrl('http://app.example.com')).toBe(false)
    expect(isSafeExternalHttpsUrl('https://user:pass@app.example.com')).toBe(false)
  })
  it('拒绝本地、私网、链路本地和 IPv6 本地地址', () => {
    for (const value of ['https://localhost', 'https://127.0.0.1:8080', 'https://10.0.0.1', 'https://192.168.1.2', 'https://169.254.169.254', 'https://[::1]']) {
      expect(isSafeExternalHttpsUrl(value)).toBe(false)
    }
  })
})
