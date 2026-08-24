import { useState } from 'react'
import { App, Button, Input, Modal, Space, Tag, Typography } from 'antd'
import { DrawerForm, ProDescriptions, ProFormTextArea } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getApplicationOnboarding, prepareApplicationCredentialApproval, queryKeys, upsertApplicationIdentityBinding } from '../../api/api'
import type { Application } from '../../types'
import QueryErrorState from '../../components/QueryErrorState'
import { formatDateTime } from '../../utils/format'

interface Props { application: Application; canManage: boolean }
interface LoginForm { redirectUris: string }

function verificationStatus(status?: string) {
  if (status === 'PASSED' || status === 'VERIFIED') return <Tag color="success">已验证</Tag>
  if (status === 'FAILED') return <Tag color="error">验证失败</Tag>
  return <Tag>待验证</Tag>
}

export default function ApplicationLogin({ application, canManage }: Props) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const onboarding = useQuery({ queryKey: queryKeys.applicationOnboarding(application.id), queryFn: () => getApplicationOnboarding(application.id), enabled: application.ssoType === 'OIDC' })
  const refresh = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: queryKeys.applicationOnboarding(application.id) }), queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] })]) }
  const configure = useMutation({
    mutationFn: async (values: LoginForm) => {
      const redirectUris = values.redirectUris.split(/\r?\n/).map((value) => value.trim()).filter(Boolean)
      const prepared = await prepareApplicationCredentialApproval(application.id, redirectUris)
      if (prepared.approvalStatus !== 'APPROVED') return { pending: true, nextAction: prepared.nextAction }
      const result = await upsertApplicationIdentityBinding(application.id, { providerKey: 'casdoor', protocol: 'OIDC', providerApplicationRef: prepared.providerApplicationRef, publicClientId: prepared.publicClientId, issuer: prepared.issuer, redirectUris, scopes: prepared.scopes, expectedConfigVersion: onboarding.data?.binding?.configVersion, credentialDeliveryMode: 'CLI' })
      return { pending: false, result }
    },
    onSuccess: async (data) => {
      setOpen(false)
      if (data.pending) {
        message.info(data.nextAction || '接入申请已提交')
      } else {
        const token = data.result?.enrollmentToken
        if (token) Modal.info({ title: '保存接入密钥', width: 680, content: <Space direction="vertical" style={{ width: '100%' }}><Input.Password value={token} readOnly visibilityToggle /><Typography.Text code copyable>{`velora-connect enroll --portal ${window.location.origin} --output /etc/${application.code}/velora`}</Typography.Text><Typography.Text type="secondary">该密钥仅显示一次，5 分钟内有效，请立即保存。</Typography.Text></Space>, okText: '已保存', maskClosable: false })
        message.success('登录设置已更新')
      }
      await refresh()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '登录设置保存失败'),
  })

  if (application.ssoType === 'URL') return <ProDescriptions column={1} dataSource={application} columns={[{ title: '登录方式', render: () => '普通链接' }, { title: '应用地址', render: () => application.launchUrl || application.homeUrl || '—' }]} />
  if (onboarding.isError) return <QueryErrorState refetch={() => void onboarding.refetch()} />
  const binding = onboarding.data?.binding
  return <>
    <ProDescriptions column={2} loading={onboarding.isLoading} dataSource={binding ?? {}} extra={canManage ? <Button type="primary" onClick={() => setOpen(true)}>{binding ? '修改登录回调' : '配置登录'}</Button> : undefined} columns={[
      { title: '统一登录', render: () => binding ? <Tag color="success">已配置</Tag> : <Tag>未配置</Tag> },
      { title: '连接检查', render: () => verificationStatus(binding?.verificationStatus) },
      { title: '登录回调地址', span: 2, render: () => binding?.redirectUris?.length ? <Space direction="vertical" size={2}>{binding.redirectUris.map((uri) => <Typography.Text key={uri} copyable>{uri}</Typography.Text>)}</Space> : '—' },
      { title: '最近检查', render: () => formatDateTime(binding?.verifiedAt) },
    ]} />
    <DrawerForm<LoginForm> key={`${application.id}-${binding?.configVersion ?? 0}`} title="登录设置" open={open} onOpenChange={setOpen} width={620} initialValues={{ redirectUris: binding?.redirectUris?.join('\n') ?? '' }} submitter={{ searchConfig: { submitText: binding ? '保存' : '提交申请', resetText: '取消' } }} onFinish={async (values) => { await configure.mutateAsync(values); return true }}>
      <ProFormTextArea name="redirectUris" label="登录回调地址" fieldProps={{ rows: 4 }} rules={[{ required: true, message: '请输入登录回调地址' }, { validator: (_: unknown, value: string) => value.split(/\r?\n/).every((uri) => /^https:\/\//i.test(uri.trim())) ? Promise.resolve() : Promise.reject(new Error('每行填写一个 HTTPS 地址')) }]} />
    </DrawerForm>
  </>
}
