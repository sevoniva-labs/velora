import { Button, Col, Row, Space, Tag, Typography } from 'antd'
import { CheckSquareOutlined, ClockCircleOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { PageContainer, ProCard, StatisticCard } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { adminDashboard, queryKeys } from '../../api/api'
import { listAccessReviews, listApprovals, listTemporaryRoleGrants } from '../../api/admin-platform'
import { useMe } from '../../auth/useMe'
import { APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE, PORTAL_MANAGE, SYSTEM_ACCESS_REVIEW_READ, SYSTEM_TEMPORARY_GRANT_READ, hasAnyPermission, hasPermission } from '../../auth/permissions'
import { usePageTitle } from '../../hooks/usePageTitle'

export default function AdminDashboard() {
  usePageTitle('工作台')
  const navigate = useNavigate()
  const me = useMe()
  const permissions = me.data?.permissions
  const roles = me.data?.roles
  const canManagePortal = hasPermission(permissions, PORTAL_MANAGE, roles)
  const canReadApprovals = hasAnyPermission(permissions, [APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE], roles)
  const canReadTemporaryGrants = hasPermission(permissions, SYSTEM_TEMPORARY_GRANT_READ, roles)
  const canReadReviews = hasPermission(permissions, SYSTEM_ACCESS_REVIEW_READ, roles)
  const dashboard = useQuery({ queryKey: queryKeys.dashboard, queryFn: adminDashboard, enabled: canManagePortal })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canReadApprovals })
  const grants = useQuery({ queryKey: ['admin', 'temporary-grants'], queryFn: listTemporaryRoleGrants, enabled: canReadTemporaryGrants })
  const reviews = useQuery({ queryKey: ['admin', 'access-reviews'], queryFn: listAccessReviews, enabled: canReadReviews })
  const pendingApprovals = (approvals.data ?? []).filter((item) => item.status === 'PENDING')
  const activeGrants = (grants.data ?? []).filter((item) => item.status === 'ACTIVE' || item.status === 'SCHEDULED')
  const openReviews = (reviews.data ?? []).filter((item) => item.status === 'OPEN')

  return <PageContainer title="工作台">
    {canManagePortal && <StatisticCard.Group direction="row">
      <StatisticCard statistic={{ title: '应用', value: dashboard.data?.applicationCount ?? 0 }} loading={dashboard.isLoading} />
      <StatisticCard statistic={{ title: '已启用', value: dashboard.data?.enabledAppCount ?? 0 }} loading={dashboard.isLoading} />
      <StatisticCard statistic={{ title: '已停用', value: dashboard.data?.disabledAppCount ?? 0 }} loading={dashboard.isLoading} />
    </StatisticCard.Group>}
    <Row gutter={[16, 16]} style={{ marginTop: canManagePortal ? 16 : 0 }}>
      {canReadApprovals && <Col xs={24} lg={8}><ProCard title="待审批" extra={<Button type="link" onClick={() => navigate('/admin/approvals')}>查看</Button>}><Space align="center"><CheckSquareOutlined style={{ fontSize: 24, color: '#1677ff' }} /><Typography.Title level={3} style={{ margin: 0 }}>{pendingApprovals.length}</Typography.Title></Space></ProCard></Col>}
      {canReadTemporaryGrants && <Col xs={24} lg={8}><ProCard title="生效中的临时授权" extra={<Button type="link" onClick={() => navigate('/admin/temporary-grants')}>查看</Button>}><Space align="center"><ClockCircleOutlined style={{ fontSize: 24, color: '#d48806' }} /><Typography.Title level={3} style={{ margin: 0 }}>{activeGrants.length}</Typography.Title></Space></ProCard></Col>}
      {canReadReviews && <Col xs={24} lg={8}><ProCard title="进行中的访问复核" extra={<Button type="link" onClick={() => navigate('/admin/access-reviews')}>查看</Button>}><Space align="center"><SafetyCertificateOutlined style={{ fontSize: 24, color: '#389e0d' }} /><Typography.Title level={3} style={{ margin: 0 }}>{openReviews.length}</Typography.Title></Space></ProCard></Col>}
    </Row>
    {canReadApprovals && pendingApprovals.length > 0 && <ProCard title="待处理事项" style={{ marginTop: 16 }}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>{pendingApprovals.slice(0, 5).map((item) => <Row key={item.id} justify="space-between" align="middle" wrap={false}><Col flex="auto"><Typography.Text>{item.summary}</Typography.Text></Col><Col><Tag color="processing">待审批</Tag></Col></Row>)}</Space>
    </ProCard>}
  </PageContainer>
}
