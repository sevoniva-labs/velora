import { Button, Col, Row, Space, Tag, Typography } from 'antd'
import { CheckSquareOutlined, ClockCircleOutlined, ExclamationCircleOutlined, SafetyCertificateOutlined, SyncOutlined, UserDeleteOutlined } from '@ant-design/icons'
import { PageContainer, ProCard, StatisticCard } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { adminDashboard, adminListAuditLogs, adminPageUsers, queryKeys } from '../../api/api'
import { getSystemReadiness, listAccessReviews, listApprovals, listTemporaryRoleGrants } from '../../api/admin-platform'
import { useMe } from '../../auth/useMe'
import { APPROVAL_REQUEST_READ, APPROVAL_TASK_DECIDE, AUDIT_READ, PORTAL_MANAGE, SYSTEM_ACCESS_REVIEW_READ, SYSTEM_TEMPORARY_GRANT_READ, SYSTEM_USER_READ, hasAnyPermission, hasPermission } from '../../auth/permissions'
import { usePageTitle } from '../../hooks/usePageTitle'
import QueryErrorState from '../../components/QueryErrorState'

interface MetricQuery { isError: boolean; isLoading: boolean; refetch: () => unknown }

function MetricValue({ query, value }: { query: MetricQuery; value: number }) {
  if (query.isError) return <Button type="link" danger onClick={() => void query.refetch()}>重试</Button>
  return <Typography.Title level={3}>{query.isLoading ? '—' : value}</Typography.Title>
}

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
  const canReadUsers = hasPermission(permissions, SYSTEM_USER_READ, roles)
  const canReadAudit = hasPermission(permissions, AUDIT_READ, roles)
  const dashboard = useQuery({ queryKey: queryKeys.dashboard, queryFn: adminDashboard, enabled: canManagePortal })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canReadApprovals })
  const grants = useQuery({ queryKey: ['admin', 'temporary-grants'], queryFn: listTemporaryRoleGrants, enabled: canReadTemporaryGrants })
  const reviews = useQuery({ queryKey: ['admin', 'access-reviews'], queryFn: listAccessReviews, enabled: canReadReviews })
  const lockedUsers = useQuery({ queryKey: ['admin', 'users', 'locked-count'], queryFn: () => adminPageUsers({ page: 1, pageSize: 1, status: 'LOCKED' }), enabled: canReadUsers })
  const failedOperations = useQuery({ queryKey: ['admin', 'audit', 'failed-count'], queryFn: () => adminListAuditLogs({ page: 1, pageSize: 1, result: 'FAILED' }), enabled: canReadAudit })
  const readiness = useQuery({ queryKey: ['system', 'readiness'], queryFn: getSystemReadiness, refetchInterval: 60_000, retry: false })
  const pendingApprovals = (approvals.data ?? []).filter((item) => item.status === 'PENDING')
  const pendingExecutions = (approvals.data ?? []).filter((item) => item.status === 'APPROVED')
  const activeGrants = (grants.data ?? []).filter((item) => item.status === 'ACTIVE' || item.status === 'SCHEDULED')
  const openReviews = (reviews.data ?? []).filter((item) => item.status === 'OPEN')

  return <PageContainer title="工作台">
    {canManagePortal && (dashboard.isError ? <QueryErrorState compact description="应用统计加载失败，请重试。" refetch={dashboard.refetch} /> : <div className="velora-dashboard-stat-grid">
      <StatisticCard className="velora-dashboard-stat-card" statistic={{ title: '应用', value: dashboard.data?.applicationCount ?? 0 }} loading={dashboard.isLoading} />
      <StatisticCard className="velora-dashboard-stat-card" statistic={{ title: '已启用', value: dashboard.data?.enabledAppCount ?? 0 }} loading={dashboard.isLoading} />
      <StatisticCard className="velora-dashboard-stat-card" statistic={{ title: '已停用', value: dashboard.data?.disabledAppCount ?? 0 }} loading={dashboard.isLoading} />
    </div>)}
    <div className="velora-dashboard-action-grid" style={{ marginTop: canManagePortal ? 16 : 0 }}>
      {canReadApprovals && <ProCard className="velora-dashboard-action-card" title="待审批" extra={<Button type="link" onClick={() => navigate('/admin/approvals')}>查看</Button>}><Space align="center"><CheckSquareOutlined className="velora-dashboard-action-icon is-primary" /><MetricValue query={approvals} value={pendingApprovals.length} /></Space></ProCard>}
      {canReadApprovals && <ProCard className="velora-dashboard-action-card" title="待执行变更" extra={<Button type="link" onClick={() => navigate('/admin/approvals')}>查看</Button>}><Space align="center"><SyncOutlined className="velora-dashboard-action-icon is-primary" /><MetricValue query={approvals} value={pendingExecutions.length} /></Space></ProCard>}
      {canReadTemporaryGrants && <ProCard className="velora-dashboard-action-card" title="正在使用的临时权限" extra={<Button type="link" onClick={() => navigate('/admin/temporary-grants')}>查看</Button>}><Space align="center"><ClockCircleOutlined className="velora-dashboard-action-icon is-warning" /><MetricValue query={grants} value={activeGrants.length} /></Space></ProCard>}
      {canReadReviews && <ProCard className="velora-dashboard-action-card" title="待完成权限复核" extra={<Button type="link" onClick={() => navigate('/admin/access-reviews')}>查看</Button>}><Space align="center"><SafetyCertificateOutlined className="velora-dashboard-action-icon is-success" /><MetricValue query={reviews} value={openReviews.length} /></Space></ProCard>}
      {canReadUsers && <ProCard className="velora-dashboard-action-card" title="已锁定用户" extra={<Button type="link" onClick={() => navigate('/admin/users')}>查看</Button>}><Space align="center"><UserDeleteOutlined className="velora-dashboard-action-icon is-warning" /><MetricValue query={lockedUsers} value={lockedUsers.data?.total ?? 0} /></Space></ProCard>}
      {canReadAudit && <ProCard className="velora-dashboard-action-card" title="失败操作" extra={<Button type="link" onClick={() => navigate('/admin/audit')}>查看</Button>}><Space align="center"><ExclamationCircleOutlined className="velora-dashboard-action-icon is-warning" /><MetricValue query={failedOperations} value={failedOperations.data?.total ?? 0} /></Space></ProCard>}
      <ProCard className="velora-dashboard-action-card" title="系统状态"><Space align="center"><SafetyCertificateOutlined className={`velora-dashboard-action-icon ${readiness.data?.status === 'UP' ? 'is-success' : 'is-warning'}`} /><Typography.Title level={3}>{readiness.isError ? '异常' : readiness.isLoading ? '检查中' : readiness.data?.status === 'UP' ? '正常' : '异常'}</Typography.Title></Space></ProCard>
    </div>
    {canReadApprovals && pendingApprovals.length > 0 && <ProCard title="待处理事项" style={{ marginTop: 16 }}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>{pendingApprovals.slice(0, 5).map((item) => <Row key={item.id} justify="space-between" align="middle" wrap={false}><Col flex="auto"><Typography.Text>{item.summary}</Typography.Text></Col><Col><Tag color="processing">待审批</Tag></Col></Row>)}</Space>
    </ProCard>}
  </PageContainer>
}
