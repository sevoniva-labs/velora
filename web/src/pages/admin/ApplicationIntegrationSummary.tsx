import { Space, Tag, Typography } from 'antd'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import type { ApplicationOnboarding } from '../../api/api'

interface Props { onboarding: ApplicationOnboarding }

function present(value?: string) { return value?.trim() || '待提供' }

export default function ApplicationIntegrationSummary({ onboarding }: Props) {
  const { application, binding, provisioningTarget, roles } = onboarding
  const directoryBasePath = `/api/v1/integrations/applications/${application.id}/directory`
  const productionCallbacks = binding?.environments.find((item) => item.key === 'PRODUCTION')?.redirectUris ?? binding?.redirectUris ?? []
  return <Space direction="vertical" size={16} style={{ width: '100%' }}>
    <ProCard title="接入方提供" className="velora-admin-section-card">
      <ProDescriptions column={2} dataSource={{}} columns={[
        { title: '应用负责人', render: () => present(application.owner) },
        { title: '生产地址', render: () => present(application.homeUrl) },
        { title: '登录回调', span: 2, render: () => productionCallbacks.length ? <Space direction="vertical" size={2}>{productionCallbacks.map((item) => <Typography.Text key={item} copyable>{item}</Typography.Text>)}</Space> : '待提供' },
        { title: '账号接收地址', span: 2, render: () => present(provisioningTarget?.endpointUrl) },
        { title: '应用角色', span: 2, render: () => roles.length ? roles.map((role) => <Tag key={role.roleKey}>{role.name}</Tag>) : '待提供' },
      ]} />
    </ProCard>
    <ProCard title="平台交付" className="velora-admin-section-card">
      <ProDescriptions column={2} dataSource={{}} columns={[
        { title: '统一登录地址', render: () => present(binding?.issuer) },
        { title: '客户端编号', render: () => binding?.publicClientId ? <Typography.Text copyable>{binding.publicClientId}</Typography.Text> : '审批通过后生成' },
        { title: '接入配置', span: 2, render: () => <Typography.Text code copyable>{`velora-connect enroll --portal ${window.location.origin} --output /etc/${application.code}/velora`}</Typography.Text> },
        { title: '组织信息接口', span: 2, render: () => <Typography.Text copyable>{`${directoryBasePath}/organization`}</Typography.Text> },
        { title: '部门接口', span: 2, render: () => <Typography.Text copyable>{`${directoryBasePath}/departments`}</Typography.Text> },
        { title: '已授权用户接口', span: 2, render: () => <Typography.Text copyable>{`${directoryBasePath}/users`}</Typography.Text> },
      ]} />
    </ProCard>
  </Space>
}
