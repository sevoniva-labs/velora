import { useEffect, useState } from 'react'
import { Button, Result, Spin } from 'antd'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { completeOIDCLogin, consumeOIDCRedirect } from '../api/api'

function callbackErrorMessage(value: unknown): string {
  const raw = value instanceof Error ? value.message : String(value || '')
  const normalized = raw.toLowerCase()
  if (normalized.includes('oidc') || normalized.includes('provider') || normalized.includes('identity')) {
    return '统一登录暂不可用，请联系系统管理员。'
  }
  return '登录验证失败，请重新发起登录。'
}

/**
 * 统一身份登录回调页。授权码由后端完成交换，浏览器不会接收访问令牌。
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
      setError('登录验证未完成，请重新发起登录。')
      return () => {
        cancelled = true
      }
    }
    void completeOIDCLogin(provider, code, state)
      .then(() => {
        if (!cancelled) navigate(consumeOIDCRedirect(), { replace: true })
      })
      .catch((reason: unknown) => {
        if (!cancelled) setError(callbackErrorMessage(reason))
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
  return <Spin fullscreen tip="正在验证登录信息…" />
}
