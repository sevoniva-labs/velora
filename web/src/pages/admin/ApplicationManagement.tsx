import { useState } from 'react'
import { Empty, Space, Tag, Typography } from 'antd'
import { PageContainer, ProCard, ProDescriptions, StatisticCard } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { adminListApplications, queryKeys } from '../../api/api'
import { APP_LIFECYCLE_COLOR, APP_LIFECYCLE_LABEL, APP_STATUS_LABEL, SSO_TYPE_LABEL, enumLabel } from '../../labels'
import { usePageTitle } from '../../hooks/usePageTitle'
import ApplicationAccess from './ApplicationAccess'

export default function ApplicationManagement() {
  const { id = '' } = useParams()
  const [tab, setTab] = useState('overview')
  const applications = useQuery({ queryKey: queryKeys.adminApplications({ detail: id }), queryFn: () => adminListApplications({ page: 1, pageSize: 500 }) })
  const application = applications.data?.items.find((item) => String(item.id) === id)
  usePageTitle(application?.name || '应用管理')
  if (!applications.isLoading && !application) return <PageContainer title="应用管理"><Empty description="应用不存在或已归档" /></PageContainer>
  return <PageContainer title={application?.name || '应用管理'} subTitle={application?.code} onBack={() => history.back()} tabList={[{ key: 'overview', tab: '概览' }, { key: 'information', tab: '应用信息' }, { key: 'login', tab: '登录配置' }, { key: 'access', tab: '角色与访问' }, { key: 'provisioning', tab: '账号同步' }, { key: 'release', tab: '验证与发布' }, { key: 'history', tab: '变更记录' }]} tabActiveKey={tab} onTabChange={setTab}>
    {tab === 'overview' && application && <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <ProCard gutter={16} wrap><StatisticCard statistic={{ title: '接入状态', value: enumLabel(APP_LIFECYCLE_LABEL, application.lifecycleStatus, '草稿') }} /><StatisticCard statistic={{ title: '运行状态', value: APP_STATUS_LABEL[application.status] }} /><StatisticCard statistic={{ title: '接入方式', value: SSO_TYPE_LABEL[application.ssoType] }} /></ProCard>
      <ProDescriptions dataSource={application} column={2} columns={[{ title: '应用名称', dataIndex: 'name' }, { title: '应用编码', dataIndex: 'code' }, { title: '接入状态', dataIndex: 'lifecycleStatus', render: (_, row) => <Tag color={APP_LIFECYCLE_COLOR[row.lifecycleStatus ?? '']}>{enumLabel(APP_LIFECYCLE_LABEL, row.lifecycleStatus, '草稿')}</Tag> }, { title: '运行状态', dataIndex: 'status', render: (_, row) => <Tag color={row.status === 'ENABLED' ? 'success' : 'default'}>{APP_STATUS_LABEL[row.status]}</Tag> }, { title: '主页地址', dataIndex: 'homeUrl', render: (_, row) => row.homeUrl || '—' }, { title: '说明', dataIndex: 'description', render: (_, row) => row.description || '—' }]} />
    </Space>}
    {tab === 'information' && application && <ProDescriptions dataSource={application} column={2} columns={[{ title: '应用名称', dataIndex: 'name' }, { title: '应用编码', dataIndex: 'code' }, { title: '接入方式', dataIndex: 'ssoType', render: (_, row) => SSO_TYPE_LABEL[row.ssoType] }, { title: '分类', render: (_, row) => row.category?.name || '未分类' }, { title: '主页地址', dataIndex: 'homeUrl' }, { title: '启动地址', dataIndex: 'launchUrl', render: (_, row) => row.launchUrl || row.homeUrl || '—' }, { title: '说明', dataIndex: 'description', span: 2, render: (_, row) => row.description || '—' }]} />}
    {tab === 'access' && <ApplicationAccess applicationId={id} />}
    {['login', 'provisioning', 'release', 'history'].includes(tab) && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Typography.Text type="secondary">正在接入现有能力</Typography.Text>} />}
  </PageContainer>
}
