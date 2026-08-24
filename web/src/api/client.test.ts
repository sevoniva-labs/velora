import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { ApiError, STEP_UP_REQUIRED_EVENT, buildQuery, apiFetch, publicErrorMessage } from './client'

describe('buildQuery', () => {
  it('过滤空值', () => {
    expect(buildQuery({ keyword: '', page: 1, featured: false })).toBe('?page=1')
  })
  it('保留有效参数', () => {
    expect(buildQuery({ keyword: 'dev', page: 2, tagIds: '1,2' })).toBe('?keyword=dev&page=2&tagIds=1%2C2')
  })
  it('空参数返回空串', () => {
    expect(buildQuery({})).toBe('')
  })
})

describe('ApiError', () => {
  it('构造与字段', () => {
    const e = new ApiError(403, 'A03001', '没有权限', 'req-1')
    expect(e.status).toBe(403)
    expect(e.code).toBe('A03001')
    expect(e.requestId).toBe('req-1')
    expect(e.message).toBe('没有权限')
  })
  it('默认错误码', () => {
    const e = new ApiError(500, '', 'boom')
    expect(e.code).toBe('A05001')
    expect(e.message).toBe('服务暂时不可用，请稍后重试。')
  })
  it('保留产品文案并隐藏实现细节', () => {
    expect(publicErrorMessage(403, '没有权限')).toBe('没有权限')
    expect(publicErrorMessage(409, 'optimistic lock conflict')).toBe('数据已发生变化，请刷新后重试。')
    expect(publicErrorMessage(500, 'dial tcp: connection refused')).toBe('服务暂时不可用，请稍后重试。')
  })
})

describe('apiFetch', () => {
  const originalFetch = globalThis.fetch

  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })
  afterEach(() => {
    globalThis.fetch = originalFetch
  })

  it('成功时解包 data', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ code: '000000', message: 'success', data: { id: 1 }, requestId: 'r1' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    const result = await apiFetch<{ id: number }>('/me')
    expect(result).toEqual({ id: 1 })
  })

  it('失败时抛 ApiError（携带后端业务码）', async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ code: 'A02001', message: '应用不存在', requestId: 'r2' }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await expect(apiFetch('/applications/999')).rejects.toMatchObject({
      status: 404,
      code: 'A02001',
      requestId: 'r2',
    })
  })

  it('写请求注入 X-CSRF-Token', async () => {
    Object.defineProperty(document, 'cookie', {
      writable: true,
      value: 'velora_csrf=csrf-token-123; velora_session=abc',
    })
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ code: '000000', data: { ok: true } }), { status: 200 }),
    )
    await apiFetch('/favorites/1', { method: 'POST' })
    const [url, init] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(url).toBe('/api/v1/favorites/1')
    const headers = (init as RequestInit).headers as Record<string, string>
    expect(headers['X-CSRF-Token']).toBe('csrf-token-123')
  })

  it('需要提升认证时通知管理端打开确认窗口', async () => {
    const listener = vi.fn()
    window.addEventListener(STEP_UP_REQUIRED_EVENT, listener)
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ code: '200027', message: 'recent MFA required' }), { status: 403 }),
    )

    await expect(apiFetch('/admin/security-sensitive-action', { method: 'POST' })).rejects.toMatchObject({ code: '200027' })
    expect(listener).toHaveBeenCalledOnce()
    window.removeEventListener(STEP_UP_REQUIRED_EVENT, listener)
  })
})
