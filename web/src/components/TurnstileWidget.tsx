import { useEffect, useRef, useState } from 'react'

declare global {
  interface Window {
    __veloraTurnstileScriptPromise?: Promise<void> | null
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
  retry?: 'auto' | 'never'
  'retry-interval'?: number
  'refresh-expired'?: 'auto' | 'manual' | 'never'
  'refresh-timeout'?: 'auto' | 'manual' | 'never'
  theme?: 'light' | 'dark' | 'auto'
  size?: 'normal' | 'compact' | 'flexible' | 'invisible'
  appearance?: 'always' | 'execute' | 'interaction-only'
}

interface TurnstileWidgetProps {
  siteKey: string
  action?: string
  onVerify: (token: string) => void
  onExpire?: () => void
  theme?: 'light' | 'dark' | 'auto'
}

const SCRIPT_ID = 'cloudflare-turnstile-script'
const SCRIPT_LOAD_TIMEOUT_MS = 8_000

/** 幂等加载 Turnstile 官方脚本（challenges.cloudflare.com）。 */
function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve()
  if (window.__veloraTurnstileScriptPromise) return window.__veloraTurnstileScriptPromise
  window.__veloraTurnstileScriptPromise = new Promise<void>((resolve, reject) => {
    let s = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null
    if (!s) {
      s = document.createElement('script')
      s.id = SCRIPT_ID
      s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
      s.async = true
    }
    const finish = (error?: Error) => {
      window.clearTimeout(timeout)
      if (error) {
        s.remove()
        window.__veloraTurnstileScriptPromise = null
        reject(error)
        return
      }
      resolve()
    }
    const timeout = window.setTimeout(() => finish(new Error('Turnstile 脚本加载超时')), SCRIPT_LOAD_TIMEOUT_MS)
    s.addEventListener('load', () => {
      if (window.turnstile) finish()
      else finish(new Error('Turnstile 未正确初始化'))
    }, { once: true })
    s.addEventListener('error', () => {
      finish(new Error('Turnstile 脚本加载失败'))
    }, { once: true })
    if (!s.isConnected) document.head.appendChild(s)
  })
  return window.__veloraTurnstileScriptPromise
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

    const clearTimedOutChallenge = () => {
      onExpire?.()
      onVerify('')
    }

    const showFailedChallenge = () => {
      onVerify('')
      setFailed(true)
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
          // Do not let an unavailable challenge retry forever. An unbounded
          // retry loop can create hundreds of bot-classified challenges and
          // lock legitimate users out. Recovery is an explicit user action.
          retry: 'never',
          'refresh-expired': 'auto',
          'refresh-timeout': 'auto',
          'error-callback': showFailedChallenge,
          'timeout-callback': clearTimedOutChallenge,
          theme,
          // This site key is configured as an Invisible widget in Cloudflare.
          // Rendering it as a visible/flexible widget makes Cloudflare reject
          // most challenges before a token is issued.
          size: 'invisible',
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
      <div ref={containerRef} data-testid="turnstile-widget" style={{ display: failed ? 'none' : undefined }} />
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
