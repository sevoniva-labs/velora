import { Suspense } from 'react'
import { Button, Result, Spin } from 'antd'
import { Outlet, useNavigate } from 'react-router-dom'
import { AdminLayout } from './layout/AdminLayout'
import { useMe } from './auth/useMe'

/** 管理后台外壳（需 velora_admin 角色）。 */
export default function AdminApp() {
  const me = useMe()
  const navigate = useNavigate()
  const isAdmin = (me.data?.roles ?? []).includes('velora_admin')

  if (me.isLoading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!isAdmin) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Result
          status="403"
          title="需要管理员权限"
          subTitle="当前账号没有管理后台访问权限。请在 Casdoor 中为该用户分配 velora_admin 角色。"
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
