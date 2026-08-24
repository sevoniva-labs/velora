import type { ReactNode } from 'react'
import { Button, Result } from 'antd'
import { PageContainer } from '@ant-design/pro-components'
import { useNavigate } from 'react-router-dom'
import { useMe } from './useMe'
import { hasAnyPermission } from './permissions'

interface RequireAdminPermissionProps {
  anyOf: string[]
  children: ReactNode
}

/** 页面级权限守卫：菜单隐藏与直达 URL 使用同一权限边界。 */
export default function RequireAdminPermission({ anyOf, children }: RequireAdminPermissionProps) {
  const me = useMe()
  const navigate = useNavigate()
  const allowed = hasAnyPermission(me.data?.permissions, anyOf, me.data?.roles)

  if (allowed) return <>{children}</>

  return (
    <PageContainer title="无权访问">
      <Result
        status="403"
        title="无权访问此页面"
        subTitle="当前账号未获得此项管理权限。"
        extra={<Button type="primary" onClick={() => navigate('/admin')}>返回工作台</Button>}
      />
    </PageContainer>
  )
}
