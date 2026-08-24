import { useState } from 'react'
import { Empty, Skeleton, Space, Tag } from 'antd'
import { PageContainer, ProCard, ProDescriptions, StatisticCard } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { adminGetApplicationProvisioningTarget, getApplicationOnboarding, queryKeys } from '../../api/api'
import { listApplicationEffectiveAccess } from '../../api/admin-platform'
import QueryErrorState from '../../components/QueryErrorState'
import { APP_LIFECYCLE_COLOR, APP_LIFECYCLE_LABEL, APP_STATUS_LABEL, enumLabel } from '../../labels'
import { usePageTitle } from '../../hooks/usePageTitle'
import ApplicationAccess from './ApplicationAccess'
import ApplicationInformation from './ApplicationInformation'
import ApplicationLogin from './ApplicationLogin'
import ApplicationProvisioning from './ApplicationProvisioning'
import ApplicationRelease from './ApplicationRelease'
import ApplicationHistory from './ApplicationHistory'
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
  const tabs = [{ key: 'overview', tab: '概览' }, ...(canManagePortal ? [{ key: 'information', tab: '应用信息' }] : []), ...(canReadIdentity ? [{ key: 'login', tab: '登录配置' }] : []), ...(canManagePortal ? [{ key: 'access', tab: '角色与访问' }] : []), ...(canReadIdentity ? [{ key: 'provisioning', tab: '账号同步' }] : []), ...(canVerify || canPublish ? [{ key: 'release', tab: '验证与发布' }] : []), ...(canReadHistory ? [{ key: 'history', tab: '变更记录' }] : [])]
  return <PageContainer title={application.name} subTitle={application.code} onBack={() => navigate('/admin/applications')} tabList={tabs} tabActiveKey={tab} onTabChange={setTab}>
    {tab === 'overview' && application && <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <ProCard gutter={16} wrap><StatisticCard statistic={{ title: '接入状态', value: enumLabel(APP_LIFECYCLE_LABEL, application.lifecycleStatus, '草稿') }} /><StatisticCard statistic={{ title: '有效用户', value: canManagePortal ? effectiveAccess.data?.length ?? 0 : '—' }} /><StatisticCard statistic={{ title: '账号同步', value: canReadIdentity ? provisioning.data?.deliveryStatus === 'HEALTHY' ? '正常' : ['DEGRADED', 'FAILED'].includes(provisioning.data?.deliveryStatus ?? '') ? '异常' : provisioning.data ? '等待同步' : '未配置' : '—' }} /></ProCard>
      <ProDescriptions dataSource={application} column={2} columns={[{ title: '应用名称', dataIndex: 'name' }, { title: '应用编码', dataIndex: 'code' }, { title: '接入状态', dataIndex: 'lifecycleStatus', render: (_, row) => <Tag color={APP_LIFECYCLE_COLOR[row.lifecycleStatus ?? '']}>{enumLabel(APP_LIFECYCLE_LABEL, row.lifecycleStatus, '草稿')}</Tag> }, { title: '运行状态', dataIndex: 'status', render: (_, row) => <Tag color={row.status === 'ENABLED' ? 'success' : 'default'}>{APP_STATUS_LABEL[row.status]}</Tag> }, { title: '主页地址', dataIndex: 'homeUrl', render: (_, row) => row.homeUrl || '—' }, { title: '说明', dataIndex: 'description', render: (_, row) => row.description || '—' }]} />
    </Space>}
    {tab === 'information' && application && <ApplicationInformation application={application} />}
    {tab === 'login' && application && <ApplicationLogin application={application} canManage={canManageIdentity} />}
    {tab === 'access' && <ApplicationAccess applicationId={id} />}
    {tab === 'provisioning' && application && <ApplicationProvisioning application={application} canManage={canManageIdentity} />}
    {tab === 'release' && application && <ApplicationRelease application={application} canVerify={canVerify} canPublish={canPublish} />}
    {tab === 'history' && <ApplicationHistory applicationId={id} />}
  </PageContainer>
}
