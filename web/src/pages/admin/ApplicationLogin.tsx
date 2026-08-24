import { useState } from 'react'
import { App, Button, Input, Modal, Space, Tag, Typography } from 'antd'
import { DrawerForm, ProDescriptions, ProFormTextArea } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getApplicationOnboarding, prepareApplicationCredentialApproval, queryKeys, upsertApplicationIdentityBinding } from '../../api/api'
import type { Application } from '../../types'
import QueryErrorState from '../../components/QueryErrorState'
import { formatDateTime } from '../../utils/format'

interface Props { application: Application; canManage: boolean }
interface LoginForm { developmentRedirectUris?: string; testRedirectUris?: string; productionRedirectUris: string }

const ENVIRONMENTS = [
  { key: 'DEVELOPMENT', name: '开发环境', field: 'developmentRedirectUris' },
  { key: 'TEST', name: '测试环境', field: 'testRedirectUris' },
  { key: 'PRODUCTION', name: '生产环境', field: 'productionRedirectUris' },
] as const

function lines(value?: string) {
  return (value ?? '').split(/\r?\n/).map((item) => item.trim()).filter(Boolean)
}

function validCallback(uri: string, allowLoopback: boolean) {
  if (/^https:\/\//i.test(uri)) return true
  return allowLoopback && /^http:\/\/(localhost|127\.0\.0\.1|\[::1\])(?::\d+)?(?:\/|$)/i.test(uri)
}

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
      const environments = ENVIRONMENTS.map((environment) => ({ key: environment.key, name: environment.name, redirectUris: lines(values[environment.field]) })).filter((environment) => environment.redirectUris.length > 0)
      const redirectUris = Array.from(new Set(environments.flatMap((environment) => environment.redirectUris)))
      const prepared = await prepareApplicationCredentialApproval(application.id, redirectUris)
      if (prepared.approvalStatus !== 'APPROVED') return { pending: true, nextAction: prepared.nextAction }
      const result = await upsertApplicationIdentityBinding(application.id, { providerKey: 'casdoor', protocol: 'OIDC', providerApplicationRef: prepared.providerApplicationRef, publicClientId: prepared.publicClientId, issuer: prepared.issuer, redirectUris, environments, scopes: prepared.scopes, expectedConfigVersion: onboarding.data?.binding?.configVersion, credentialDeliveryMode: 'CLI' })
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

  if (application.ssoType === 'URL') return <ProDescriptions className="velora-admin-section-card" column={1} dataSource={application} columns={[{ title: '登录方式', render: () => '普通链接' }, { title: '应用地址', render: () => application.launchUrl || application.homeUrl || '—' }]} />
  if (onboarding.isError) return <QueryErrorState refetch={() => void onboarding.refetch()} />
  const binding = onboarding.data?.binding
  const environmentValues = Object.fromEntries(ENVIRONMENTS.map((environment) => [environment.field, binding?.environments.find((item) => item.key === environment.key)?.redirectUris.join('\n') ?? (environment.key === 'PRODUCTION' ? binding?.redirectUris.join('\n') ?? '' : '')])) as unknown as LoginForm
  return <>
    <ProDescriptions className="velora-admin-section-card" column={2} loading={onboarding.isLoading} dataSource={binding ?? {}} extra={canManage ? <Button type="primary" onClick={() => setOpen(true)}>{binding ? '修改登录回调' : '配置登录'}</Button> : undefined} columns={[
      { title: '统一登录', render: () => binding ? <Tag color="success">已配置</Tag> : <Tag>未配置</Tag> },
      { title: '连接检查', render: () => verificationStatus(binding?.verificationStatus) },
      { title: '登录回调地址', span: 2, render: () => binding?.redirectUris?.length ? <Space direction="vertical" size={2}>{binding.redirectUris.map((uri) => <Typography.Text key={uri} copyable>{uri}</Typography.Text>)}</Space> : '—' },
      { title: '接入环境', render: () => binding?.environments?.length ? binding.environments.map((environment) => <Tag key={environment.key}>{environment.name}</Tag>) : binding ? <Tag>生产环境</Tag> : '—' },
      { title: '最近检查', render: () => formatDateTime(binding?.verifiedAt) },
    ]} />
    <DrawerForm<LoginForm> key={`${application.id}-${binding?.configVersion ?? 0}`} title="登录设置" open={open} onOpenChange={setOpen} width={620} initialValues={environmentValues} submitter={{ searchConfig: { submitText: binding ? '保存' : '提交申请', resetText: '取消' } }} onFinish={async (values) => { await configure.mutateAsync(values); return true }}>
      <ProFormTextArea name="developmentRedirectUris" label="开发环境回调地址" fieldProps={{ rows: 2, placeholder: 'HTTPS 地址；本机可使用 http://localhost' }} rules={[{ validator: (_: unknown, value?: string) => lines(value).every((uri) => validCallback(uri, true)) ? Promise.resolve() : Promise.reject(new Error('请输入 HTTPS 地址；仅本机允许 HTTP')) }]} />
      <ProFormTextArea name="testRedirectUris" label="测试环境回调地址" fieldProps={{ rows: 2, placeholder: '每行一个 HTTPS 地址' }} rules={[{ validator: (_: unknown, value?: string) => lines(value).every((uri) => validCallback(uri, false)) ? Promise.resolve() : Promise.reject(new Error('每行填写一个 HTTPS 地址')) }]} />
      <ProFormTextArea name="productionRedirectUris" label="生产环境回调地址" fieldProps={{ rows: 3, placeholder: '每行一个 HTTPS 地址' }} rules={[{ required: true, message: '请输入生产环境回调地址' }, { validator: (_: unknown, value?: string) => lines(value).every((uri) => validCallback(uri, false)) ? Promise.resolve() : Promise.reject(new Error('每行填写一个 HTTPS 地址')) }]} />
    </DrawerForm>
  </>
}
