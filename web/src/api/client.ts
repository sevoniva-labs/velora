// 小型 fetch 封装：统一 baseURL、凭证、JSON 处理与 Velora 统一返回结构。
// 后端契约：{"code":"000000","message":"success","data":{...},"requestId":"..."}

/** 统一 API 错误：携带 HTTP 状态码与后端稳定业务码。 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId?: string

  constructor(status: number, code: string, message: string, requestId?: string) {
    super(message || `请求失败（HTTP ${status}）`)
    this.name = 'ApiError'
    this.status = status
    this.code = code || 'A05001'
    this.requestId = requestId
  }
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

  const data = await parseBody<VeloraEnvelope>(res)
  if (!res.ok) {
    throw new ApiError(res.status, data?.code ?? 'A05001', data?.message ?? '', data?.requestId)
  }
  // 统一返回结构取 data 字段。
  return (data?.data ?? undefined) as T
}

interface VeloraEnvelope {
  code: string
  message?: string
  data?: unknown
  requestId?: string
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
