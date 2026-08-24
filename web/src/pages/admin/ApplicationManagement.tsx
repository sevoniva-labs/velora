import { useState } from 'react'
import { Empty, Skeleton, Space, Tag } from 'antd'
import { PageContainer, ProDescriptions, StatisticCard } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { adminGetApplicationProvisioningTarget, getApplicationOnboarding, queryKeys } from '../../api/api'
import { listApplicationEffectiveAccess } from '../../api/admin-platform'
import QueryErrorState from '../../components/QueryErrorState'
import { APP_LIFECYCLE_COLOR, APP_LIFECYCLE_LABEL, enumLabel } from '../../labels'
import { usePageTitle } from '../../hooks/usePageTitle'
import ApplicationAccess from './ApplicationAccess'
import ApplicationInformation from './ApplicationInformation'
import ApplicationLogin from './ApplicationLogin'
import ApplicationProvisioning from './ApplicationProvisioning'
import ApplicationRelease from './ApplicationRelease'
import ApplicationHistory from './ApplicationHistory'
import ApplicationIntegrationSummary from './ApplicationIntegrationSummary'
import { AUDIT_READ, IDENTITY_MANAGE, IDENTITY_READ, IDENTITY_VERIFY, PORTAL_MANAGE, PORTAL_PUBLISH, hasPermission } from '../../auth/permissions'
import { useMe } from '../../auth/useMe'

export default function ApplicationManagement() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const me = useMe()
  const canManagePortal = hasPermission(me.data?.permissions, PORTAL_MANAGE, me.data?.roles)
  const canReadIdentity = canManagePortal || hasPermission(me.data?.permissions, IDENTITY_READ, me.data?.roles) || hasPermission(me.data?.permissions, IDENTITY_MANAGE, me.data?.roles) || hasPermission(me.data?.permissions, IDENTITY_VERIFY, me.data?.roles)
  const canManageIdentity = canManagePortal || hasPermission(me.data?.permissions, IDENTITY_MANAGE, me.data?.roles)
  const canVerify = hasPermission(me.data?.permissions, IDENTITY_VERIFY, me.data?.roles)
  const canPublish = hasPermission(me.data?.permissions, PORTAL_PUBLISH, me.data?.roles)
  const canReadHistory = hasPermission(me.data?.permissions, AUDIT_READ, me.data?.roles)
  const [tab, setTab] = useState('overview')
  const onboarding = useQuery({ queryKey: queryKeys.applicationOnboarding(id), queryFn: () => getApplicationOnboarding(id), enabled: Boolean(id) })
  const application = onboarding.data?.application
  const effectiveAccess = useQuery({ queryKey: ['admin', 'applications', id, 'effective-access'], queryFn: () => listApplicationEffectiveAccess(id), enabled: Boolean(id) && canManagePortal })
  const provisioning = useQuery({ queryKey: queryKeys.applicationProvisioningTarget(id), queryFn: () => adminGetApplicationProvisioningTarget(id), enabled: Boolean(id) && canReadIdentity })
  usePageTitle(application?.name || '应用管理')
  if (onboarding.isLoading) return <PageContainer title="应用管理" onBack={() => navigate('/admin/applications')}><div className="velora-admin-page-card"><Skeleton active /></div></PageContainer>
  if (onboarding.isError) return <PageContainer title="应用管理" onBack={() => navigate('/admin/applications')}><QueryErrorState refetch={() => void onboarding.refetch()} /></PageContainer>
  if (!application) return <PageContainer title="应用管理" onBack={() => navigate('/admin/applications')}><Empty description="应用不存在或已归档" /></PageContainer>
  const tabs = [{ key: 'overview', tab: '概览' }, ...(canReadIdentity ? [{ key: 'integration', tab: '接入资料' }] : []), ...(canManagePortal ? [{ key: 'information', tab: '基本信息' }] : []), ...(canManagePortal ? [{ key: 'roles', tab: '应用角色' }, { key: 'access', tab: '使用范围' }] : []), ...(canReadIdentity ? [{ key: 'provisioning', tab: '账号下发' }, { key: 'login', tab: '登录设置' }] : []), ...(canVerify || canPublish ? [{ key: 'release', tab: '上线检查' }] : []), ...(canReadHistory ? [{ key: 'history', tab: '操作记录' }] : [])]
  return <PageContainer className="velora-application-management" title={application.name} subTitle={application.code} onBack={() => navigate('/admin/applications')} tabList={tabs} tabActiveKey={tab} onTabChange={setTab}>
    {tab === 'overview' && application && <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <div className="velora-application-stat-grid"><StatisticCard className="velora-application-stat-card" statistic={{ title: '接入状态', value: enumLabel(APP_LIFECYCLE_LABEL, application.lifecycleStatus, '草稿') }} /><StatisticCard className="velora-application-stat-card" statistic={{ title: '可使用人数', value: canManagePortal ? effectiveAccess.data?.length ?? 0 : '—' }} /><StatisticCard className="velora-application-stat-card" statistic={{ title: '账号下发', value: canReadIdentity ? provisioning.data?.deliveryStatus === 'HEALTHY' ? '正常' : ['DEGRADED', 'FAILED'].includes(provisioning.data?.deliveryStatus ?? '') ? '异常' : provisioning.data ? '等待下发' : '未配置' : '—' }} /></div>
      <ProDescriptions className="velora-application-overview" dataSource={application} column={{ xs: 1, sm: 1, md: 2 }} columns={[{ title: '应用名称', dataIndex: 'name' }, { title: '应用编码', dataIndex: 'code' }, { title: '接入状态', dataIndex: 'lifecycleStatus', render: (_, row) => <Tag color={APP_LIFECYCLE_COLOR[row.lifecycleStatus ?? '']}>{enumLabel(APP_LIFECYCLE_LABEL, row.lifecycleStatus, '草稿')}</Tag> }, { title: '使用状态', dataIndex: 'status', render: (_, row) => <Tag color={row.status === 'ENABLED' ? 'success' : 'default'}>{row.status === 'ENABLED' ? '可用' : '已停用'}</Tag> }, { title: '应用地址', dataIndex: 'homeUrl', render: (_, row) => row.homeUrl || '—' }, { title: '用途', dataIndex: 'description', render: (_, row) => row.description || '—' }]} />
    </Space>}
    {tab === 'integration' && onboarding.data && <ApplicationIntegrationSummary onboarding={onboarding.data} />}
    {tab === 'information' && application && <ApplicationInformation application={application} />}
    {tab === 'login' && application && <ApplicationLogin application={application} canManage={canManageIdentity} />}
    {tab === 'roles' && <ApplicationAccess applicationId={id} view="roles" />}
    {tab === 'access' && <ApplicationAccess applicationId={id} view="access" />}
    {tab === 'provisioning' && application && <ApplicationProvisioning application={application} canManage={canManageIdentity} hasIdentityBinding={Boolean(onboarding.data?.binding)} />}
    {tab === 'release' && application && <ApplicationRelease application={application} canVerify={canVerify} canPublish={canPublish} />}
    {tab === 'history' && <ApplicationHistory applicationId={id} />}
  </PageContainer>
}
