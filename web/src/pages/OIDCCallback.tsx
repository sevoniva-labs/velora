import { useEffect, useState } from 'react'
import { Button, Result, Spin } from 'antd'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { completeOIDCLogin, consumeOIDCRedirect } from '../api/api'

/**
 * Casdoor redirect target. The authorization code is exchanged by the
 * backend; the browser never receives an access/refresh token.
 */
export default function OIDCCallback() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    const provider = params.get('provider') || 'casdoor'
    const code = params.get('code') || ''
    const state = params.get('state') || ''
    const providerError = params.get('error_description') || params.get('error') || ''
    if (providerError || !code || !state) {
      setError(providerError || '统一登录回调参数不完整')
      return () => {
        cancelled = true
      }
    }
    void completeOIDCLogin(provider, code, state)
      .then(() => {
        if (!cancelled) navigate(consumeOIDCRedirect(), { replace: true })
      })
      .catch((reason: unknown) => {
        if (!cancelled) setError(reason instanceof Error ? reason.message : '统一登录失败，请重新发起登录')
      })
    return () => {
      cancelled = true
    }
  }, [navigate, params])

  if (error) {
    return (
      <Result
        status="error"
        title="统一登录失败"
        subTitle={error}
        extra={<Button type="primary" onClick={() => navigate('/login', { replace: true })}>返回登录</Button>}
      />
    )
  }
  return <Spin fullscreen tip="正在完成统一登录…" />
}
