import { Suspense } from 'react'
import { Button, Result, Spin } from 'antd'
import { Outlet, useNavigate } from 'react-router-dom'
import { AdminLayout } from './layout/AdminLayout'
import { useMe } from './auth/useMe'
import { canAccessAdmin } from './auth/permissions'

/** 管理后台外壳（由服务端权限集合控制）。 */
export default function AdminApp() {
  const me = useMe()
  const navigate = useNavigate()
  const canEnterAdmin = canAccessAdmin(me.data?.permissions, me.data?.roles)

  if (me.isLoading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!canEnterAdmin) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Result
          status="403"
          title="需要管理员权限"
          subTitle="当前账号没有访问管理后台的权限，请联系平台管理员开通相应权限。"
          extra={
            <Button type="primary" onClick={() => navigate('/home')}>
              返回门户
            </Button>
          }
        />
      </div>
    )
  }

  return (
    <AdminLayout>
      <Suspense
        fallback={
          <div style={{ padding: '48px 0', textAlign: 'center' }}>
            <Spin size="large" />
          </div>
        }
      >
        <Outlet />
      </Suspense>
    </AdminLayout>
  )
}
