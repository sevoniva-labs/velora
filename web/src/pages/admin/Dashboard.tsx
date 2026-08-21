import { useQuery } from '@tanstack/react-query'
import { Alert, Button, Col, Row } from 'antd'
import {
  AppstoreOutlined,
  CheckCircleOutlined,
  HeartOutlined,
  PauseCircleOutlined,
  RocketOutlined,
  TagsOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'
import AdminPageHead from '../../components/AdminPageHead'
import { adminDashboard, queryKeys } from '../../api/api'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMe } from '../../auth/useMe'
import { useNavigate } from 'react-router-dom'
import { IDENTITY_READ, PORTAL_MANAGE, hasPermission } from '../../auth/permissions'

export default function AdminDashboard() {
  usePageTitle('门户概览')
  const me = useMe()
  const navigate = useNavigate()
  const canManagePortal = hasPermission(me.data?.permissions, PORTAL_MANAGE, me.data?.roles)
  const canReadIdentity = hasPermission(me.data?.permissions, IDENTITY_READ, me.data?.roles)

  const { data, isError, refetch } = useQuery({ queryKey: queryKeys.dashboard, queryFn: adminDashboard, enabled: Boolean(canManagePortal) })

  const stats = [
    { title: '应用总数', value: data?.applicationCount ?? '-', icon: <AppstoreOutlined />, color: '#1677FF' },
    { title: '启用应用', value: data?.enabledAppCount ?? '-', icon: <CheckCircleOutlined />, color: '#52C41A' },
    { title: '停用应用', value: data?.disabledAppCount ?? '-', icon: <PauseCircleOutlined />, color: '#98A2B3' },
    { title: '应用分类', value: data?.categoryCount ?? '-', icon: <UnorderedListOutlined />, color: '#722ED1' },
    { title: '应用标签', value: data?.tagCount ?? '-', icon: <TagsOutlined />, color: '#FA8C16' },
    { title: '收藏总数', value: data?.favoriteCount ?? '-', icon: <HeartOutlined />, color: '#FA541C' },
    { title: '累计启动', value: data?.totalLaunches ?? '-', icon: <RocketOutlined />, color: '#13C2C2' },
  ]

  return (
    <div>
      <AdminPageHead title="门户概览" />

      {!canManagePortal && canReadIdentity ? <Alert type="info" showIcon message="当前账号为身份管理角色" description="你可以管理身份接入、验证和身份控制台入口；应用目录与发布由应用管理员负责。" action={<Button onClick={() => navigate('/admin/identity')}>进入身份接入</Button>} style={{ marginBottom: 16 }} /> : null}
      {!canManagePortal && !canReadIdentity ? <Alert type="info" showIcon message="当前账号没有门户管理权限" description="请从左侧进入你被授权的管理功能。" style={{ marginBottom: 16 }} /> : null}

      {canManagePortal && isError ? (
        <QueryErrorState refetch={refetch} />
      ) : canManagePortal ? (
        <Row gutter={[16, 16]}>
          {stats.map((s) => (
            <Col xs={12} md={8} lg={6} key={s.title}>
              <div className="velora-admin-stat">
                <span
                  className="velora-admin-stat-icon"
                  style={{ color: s.color, background: `color-mix(in srgb, ${s.color} 10%, white)` }}
                >
                  {s.icon}
                </span>
                <div className="velora-admin-stat-text">
                  <span className="velora-admin-stat-value">{s.value}</span>
                  <span className="velora-admin-stat-label">{s.title}</span>
                </div>
              </div>
            </Col>
          ))}
        </Row>
      ) : null}
    </div>
  )
}
