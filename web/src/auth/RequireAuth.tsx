import { useEffect } from 'react'
import type { ReactNode } from 'react'
import { Button, Result, Spin } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import { useMe } from './useMe'
import { ApiError } from '../api/client'

/** 登录守卫：未登录（/me 返回 401）跳 /login，并携带 redirect 以便登录后跳回。 */
export default function RequireAuth({ children }: { children: ReactNode }) {
  const location = useLocation()
  const navigate = useNavigate()
  const me = useMe()

  const unauthorized = me.isError && me.error instanceof ApiError && me.error.status === 401

  useEffect(() => {
    if (unauthorized) {
      const target = `${location.pathname}${location.search}`
      navigate(`/login?redirect=${encodeURIComponent(target)}`, { replace: true })
    }
  }, [unauthorized, location.pathname, location.search, navigate])

  if (me.isLoading || unauthorized) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Spin size="large" tip="正在验证登录状态…">
          <div style={{ minWidth: 160, minHeight: 80 }} />
        </Spin>
      </div>
    )
  }

  if (me.isError) {
    const message = me.error instanceof ApiError ? me.error.message : '网络异常，请检查服务是否可用'
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Result
          status="warning"
          title="无法获取登录状态"
          subTitle={message}
          extra={
            <Button type="primary" onClick={() => void me.refetch()}>
              重试
            </Button>
          }
        />
      </div>
    )
  }

  return <>{children}</>
}
