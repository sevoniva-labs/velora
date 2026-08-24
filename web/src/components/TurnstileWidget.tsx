import { useEffect, useRef, useState } from 'react'

declare global {
  interface Window {
    turnstile?: {
      render: (el: HTMLElement, opts: TurnstileRenderOptions) => string
      reset: (widgetId?: string) => void
      remove: (widgetId: string) => void
    }
  }
}

interface TurnstileRenderOptions {
  sitekey: string
  action?: string
  callback: (token: string) => void
  'error-callback'?: (errorCode?: string) => void
  'expired-callback'?: () => void
  'timeout-callback'?: () => void
  theme?: 'light' | 'dark' | 'auto'
  size?: 'normal' | 'compact' | 'flexible'
  appearance?: 'always' | 'execute' | 'interaction-only'
}

interface TurnstileWidgetProps {
  siteKey: string
  action?: string
  onVerify: (token: string) => void
  onExpire?: () => void
  theme?: 'light' | 'dark' | 'auto'
}

let scriptPromise: Promise<void> | null = null
const SCRIPT_ID = 'cloudflare-turnstile-script'
const SCRIPT_LOAD_TIMEOUT_MS = 8_000

/** 幂等加载 Turnstile 官方脚本（challenges.cloudflare.com）。 */
function loadTurnstileScript(): Promise<void> {
  if (scriptPromise) return scriptPromise
  scriptPromise = new Promise<void>((resolve, reject) => {
    if (window.turnstile) {
      resolve()
      return
    }
    document.getElementById(SCRIPT_ID)?.remove()
    const s = document.createElement('script')
    s.id = SCRIPT_ID
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    s.async = true
    const finish = (error?: Error) => {
      window.clearTimeout(timeout)
      s.onload = null
      s.onerror = null
      if (error) {
        s.remove()
        scriptPromise = null
        reject(error)
        return
      }
      resolve()
    }
    const timeout = window.setTimeout(() => finish(new Error('Turnstile 脚本加载超时')), SCRIPT_LOAD_TIMEOUT_MS)
    s.onload = () => {
      if (window.turnstile) finish()
      else finish(new Error('Turnstile 未正确初始化'))
    }
    s.onerror = () => {
      scriptPromise = null // 允许重试
      finish(new Error('Turnstile 脚本加载失败'))
    }
    document.head.appendChild(s)
  })
  return scriptPromise
}

/**
 * Cloudflare Turnstile 人机验证 widget（显式渲染）。
 * 验证通过后回调携带一次性 token，由登录请求随 body 提交，服务端校验。
 */
export default function TurnstileWidget({ siteKey, action = 'login', onVerify, onExpire, theme = 'auto' }: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const widgetIdRef = useRef<string | null>(null)
  const [failed, setFailed] = useState(false)
  const [retryAttempt, setRetryAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    let widgetId: string | null = null
    const el = containerRef.current
    setFailed(false)
    onVerify('')

    const failVerification = () => {
      onVerify('')
      setFailed(true)
    }

    const resetTimedOutChallenge = () => {
      onExpire?.()
      onVerify('')
      if (!widgetId || !window.turnstile) {
        setRetryAttempt((attempt) => attempt + 1)
        return
      }
      try {
        window.turnstile.reset(widgetId)
      } catch {
        setRetryAttempt((attempt) => attempt + 1)
      }
    }

    loadTurnstileScript()
      .then(() => {
        if (cancelled || !el || !window.turnstile) return
        widgetId = window.turnstile.render(el, {
          sitekey: siteKey,
          action,
          callback: (token: string) => {
            onVerify(token)
          },
          'expired-callback': () => {
            onExpire?.()
            onVerify('')
          },
          'error-callback': failVerification,
          // A challenge timeout is recoverable. Reset it in place instead of
          // hiding the widget and leaving the login button permanently disabled.
          'timeout-callback': resetTimedOutChallenge,
          theme,
          size: 'flexible',
          appearance: 'interaction-only',
        })
        widgetIdRef.current = widgetId
      })
      .catch(() => {
        if (!cancelled) setFailed(true)
      })

    return () => {
      cancelled = true
      if (widgetIdRef.current && window.turnstile) {
        try {
          window.turnstile.remove(widgetIdRef.current)
        } catch {
          // widget 未完成渲染时 remove 可能抛错，忽略
        }
        widgetIdRef.current = null
      }
      // 清空容器：StrictMode 双挂载 / key 重挂载时避免残留重复 widget
      if (el) el.innerHTML = ''
    }
    // siteKey/action 变化即重建，避免把旧 action 的 token 发给后端。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siteKey, action, retryAttempt])

  return (
    <div>
      <div ref={containerRef} data-testid="turnstile-widget" style={{ display: failed ? 'none' : undefined, minHeight: 65 }} />
      {failed && (
        <div role="alert" style={{ minHeight: 32, color: '#667085', fontSize: 13, lineHeight: '32px' }}>
          安全验证暂不可用，
          <button
            type="button"
            onClick={() => setRetryAttempt((attempt) => attempt + 1)}
            style={{ border: 0, padding: 0, color: '#1677ff', background: 'transparent', cursor: 'pointer' }}
          >
            重新加载
          </button>
        </div>
      )}
    </div>
  )
}
