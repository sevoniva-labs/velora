// 小型 fetch 封装：统一 baseURL、凭证、JSON 处理与 Velora 统一返回结构。
// 后端契约：{"code":"000000","message":"success","data":{...},"request_id":"..."}

/** 统一 API 错误：携带 HTTP 状态码与后端稳定业务码。 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId?: string

  constructor(status: number, code: string, message: string, requestId?: string) {
    super(publicErrorMessage(status, message))
    this.name = 'ApiError'
    this.status = status
    this.code = code || 'A05001'
    this.requestId = requestId
  }
}

/**
 * 后端错误详情用于日志定位；界面只展示稳定、可操作的产品文案。
 * 已经产品化的中文信息原样保留，英文实现细节不直接暴露给管理员。
 */
export function publicErrorMessage(status: number, message: string): string {
  const normalized = message.trim()
  if (/[\u3400-\u9fff]/u.test(normalized)) return normalized
  if (status === 400 || status === 422) return '提交内容不符合要求，请检查后重试。'
  if (status === 401) return '登录状态已失效，请重新登录。'
  if (status === 403) return '当前账号无权执行此操作。'
  if (status === 404) return '目标记录不存在或已被删除。'
  if (status === 409) return '数据已发生变化，请刷新后重试。'
  if (status === 412 || status === 423) return '当前状态不允许执行此操作，请刷新后重试。'
  if (status === 429) return '操作过于频繁，请稍后重试。'
  return status >= 500 ? '服务暂时不可用，请稍后重试。' : '操作失败，请稍后重试。'
}

/** 从 document.cookie 读取 velora_csrf（写请求双提交用）。 */
export function getCsrfToken(): string {
  const prefix = 'velora_csrf='
  const cookie = document.cookie
    .split(';')
    .map((item) => item.trim())
    .find((item) => item.startsWith(prefix))
  return cookie ? decodeURIComponent(cookie.slice(prefix.length)) : ''
}

const BASE_URL = '/api/v1'

export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'

export interface RequestOptions {
  method?: HttpMethod
  body?: unknown
}

const WRITE_METHODS: ReadonlySet<string> = new Set(['POST', 'PATCH', 'PUT', 'DELETE'])

// 会话过期（401）处理：自动跳登录页并携带回跳地址。
// 模块级标志防止多个并发请求同时触发多次跳转。
let redirectingToLogin = false
export const STEP_UP_REQUIRED_EVENT = 'velora:step-up-required'

function handleUnauthorized(path: string): void {
  // 已在登录页 / 登录相关端点自身的 401（账号/密码错误）不触发跳转，避免死循环与闪烁。
  if (redirectingToLogin) return
  if (!shouldRedirectUnauthorized(path, window.location.pathname)) return
  redirectingToLogin = true
  const current = window.location.pathname + window.location.search
  const target = `/login?redirect=${encodeURIComponent(current === '/' ? '' : current)}`
  window.location.assign(target)
}

/** 登录事务自身的 401 必须留在当前页面，以便用户修正凭据或 MFA 后重试。 */
export function shouldRedirectUnauthorized(path: string, pathname: string): boolean {
  if (pathname.startsWith('/login')) return false
  return !(path.startsWith('/auth/login') || path === '/auth/oidc/login' || path === '/auth/step-up' || path === '/auth/wechat/complete')
}

/** 发起请求；非 2xx 抛 ApiError。写请求自动注入 X-CSRF-Token。 */
export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const method = options.method ?? 'GET'
  const headers: Record<string, string> = {}
  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (WRITE_METHODS.has(method)) {
    const csrf = getCsrfToken()
    if (csrf) headers['X-CSRF-Token'] = csrf
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    credentials: 'include',
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  })

  // 会话失效（未登录/已过期/已吊销）：统一跳登录页。
  if (res.status === 401) {
    handleUnauthorized(path)
  }

  const data = await parseBody<VeloraEnvelope>(res)
  if (!res.ok) {
    if (data?.code === '200027' && path !== '/auth/step-up') {
      window.dispatchEvent(new CustomEvent(STEP_UP_REQUIRED_EVENT))
    }
    throw new ApiError(res.status, data?.code ?? 'A05001', data?.message ?? '', data?.requestId ?? data?.request_id)
  }
  // 统一返回结构取 data 字段。
  return snakeToCamel(data?.data ?? undefined) as T
}

interface VeloraEnvelope {
  code: string
  message?: string
  data?: unknown
  requestId?: string
  request_id?: string
}

async function parseBody<T>(res: Response): Promise<T | undefined> {
  const text = await res.text()
  if (!text) return undefined
  try {
    return JSON.parse(text) as T
  } catch {
    return undefined
  }
}

/** 拼接查询参数（自动过滤空值）。 */
export function buildQuery(params: Record<string, string | number | boolean | undefined>): string {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '' && v !== false) {
      qs.set(k, String(v))
    }
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

/**
 * Kratos 的 protojson 编码使用 proto 字段名（snake_case），而 Web 领域模型
 * 保持 camelCase。集中在传输层做递归转换，避免每个页面重复处理字段。
 */
export function snakeToCamel<T>(value: T): T {
  if (Array.isArray(value)) return value.map((item) => snakeToCamel(item)) as T
  if (value && typeof value === 'object') {
    const result: Record<string, unknown> = {}
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      result[key.replace(/_([a-z])/g, (_, letter: string) => letter.toUpperCase())] = snakeToCamel(item)
    }
    return result as T
  }
  return value
}
