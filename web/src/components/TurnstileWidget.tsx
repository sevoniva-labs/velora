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
  callback: (token: string) => void
  'error-callback'?: () => void
  'expired-callback'?: () => void
  theme?: 'light' | 'dark' | 'auto'
  size?: 'normal' | 'compact' | 'flexible'
}

interface TurnstileWidgetProps {
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  theme?: 'light' | 'dark' | 'auto'
}

let scriptPromise: Promise<void> | null = null

/** 幂等加载 Turnstile 官方脚本（challenges.cloudflare.com）。 */
function loadTurnstileScript(): Promise<void> {
  if (scriptPromise) return scriptPromise
  scriptPromise = new Promise<void>((resolve, reject) => {
    if (window.turnstile) {
      resolve()
      return
    }
    const s = document.createElement('script')
    s.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => {
      scriptPromise = null // 允许重试
      reject(new Error('Turnstile 脚本加载失败'))
    }
    document.head.appendChild(s)
  })
  return scriptPromise
}

/**
 * Cloudflare Turnstile 人机验证 widget（显式渲染）。
 * 验证通过后回调携带一次性 token，由登录请求随 body 提交，服务端校验。
 */
export default function TurnstileWidget({ siteKey, onVerify, onExpire, theme = 'auto' }: TurnstileWidgetProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const widgetIdRef = useRef<string | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let cancelled = false
    let widgetId: string | null = null

    loadTurnstileScript()
      .then(() => {
        if (cancelled || !containerRef.current || !window.turnstile) return
        widgetId = window.turnstile.render(containerRef.current, {
          sitekey: siteKey,
          callback: (token: string) => onVerify(token),
          'expired-callback': () => {
            onExpire?.()
            onVerify('')
          },
          'error-callback': () => setFailed(true),
          theme,
          size: 'flexible',
        })
        widgetIdRef.current = widgetId
      })
      .catch(() => setFailed(true))

    return () => {
      cancelled = true
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.remove(widgetIdRef.current)
        widgetIdRef.current = null
      }
    }
    // siteKey 变化即重建（组件按登录页生命周期挂载一次，防重复渲染）
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [siteKey])

  if (failed) {
    return <div style={{ color: '#fa541c', fontSize: 13 }}>人机验证组件加载失败，请刷新页面重试</div>
  }
  return <div ref={containerRef} data-testid="turnstile-widget" style={{ minHeight: 65 }} />
}
