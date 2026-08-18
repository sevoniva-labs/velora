import { Suspense } from 'react'
import { Spin } from 'antd'
import { Outlet } from 'react-router-dom'
import { PortalLayout } from './layout/PortalLayout'

/** 普通用户门户外壳。 */
export default function App() {
  return (
    <PortalLayout>
      <Suspense
        fallback={
          <div style={{ padding: '48px 0', textAlign: 'center' }}>
            <Spin size="large" />
          </div>
        }
      >
        <Outlet />
      </Suspense>
    </PortalLayout>
  )
}
