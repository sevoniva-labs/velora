import { useMemo, useState } from 'react'
import { Alert, App as AntdApp, Button, Card, Descriptions, Form, Input, List, Select, Space, Steps, Tag, Typography } from 'antd'
import { ExportOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AdminPageHead from '../../components/AdminPageHead'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMe } from '../../auth/useMe'
import {
  adminListApplications,
  getApplicationOnboarding,
  getIdentityConsoleLink,
  getIdentityOverview,
  publishApplication,
  queryKeys,
  submitApplicationPublish,
  upsertApplicationIdentityBinding,
  verifyApplicationIdentity,
} from '../../api/api'

const { Paragraph, Text } = Typography

export default function IdentityIntegrations() {
  usePageTitle('身份与单点登录')
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const me = useMe()
  const [selectedId, setSelectedId] = useState<string>()
  const [step, setStep] = useState(0)
  const [form] = Form.useForm()
  const canRead = me.data?.permissions.includes('iam.integration.read') || me.data?.permissions.includes('system.role.manage') || me.data?.roles.includes('system_admin')
  const { data: overview, isError: overviewError, refetch: refetchOverview } = useQuery({ queryKey: queryKeys.identityOverview, queryFn: getIdentityOverview, enabled: Boolean(canRead) })
  const { data: apps, isLoading: appsLoading } = useQuery({ queryKey: queryKeys.adminApplications({ onboarding: true }), queryFn: () => adminListApplications({ page: 1, pageSize: 500 }), enabled: Boolean(canRead) })
  const selected = useMemo(() => apps?.items.find((item) => String(item.id) === selectedId), [apps, selectedId])
  const onboarding = useQuery({ queryKey: queryKeys.applicationOnboarding(selectedId ?? ''), queryFn: () => getApplicationOnboarding(selectedId!), enabled: Boolean(selectedId) })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.applicationOnboarding(selectedId ?? '') })
    void queryClient.invalidateQueries({ queryKey: queryKeys.adminApplications() })
    void queryClient.invalidateQueries({ queryKey: queryKeys.applications() })
  }
  const bindingMutation = useMutation({
    mutationFn: (values: { providerApplicationRef: string; publicClientId: string; issuer: string; redirectUris: string }) => upsertApplicationIdentityBinding(selectedId!, { providerKey: 'casdoor', protocol: selected?.ssoType ?? 'OIDC', providerApplicationRef: values.providerApplicationRef, publicClientId: values.publicClientId, issuer: values.issuer, redirectUris: values.redirectUris.split(/\r?\n/).map((value) => value.trim()).filter(Boolean), expectedConfigVersion: onboarding.data?.binding?.configVersion }),
    onSuccess: () => { message.success('身份绑定已保存，等待验证'); invalidate(); setStep(3) },
    onError: (err) => message.error(err instanceof Error ? err.message : '身份绑定保存失败'),
  })
  const verifyMutation = useMutation({
    mutationFn: () => verifyApplicationIdentity(selectedId!, onboarding.data?.binding?.configVersion),
    onSuccess: (result) => { message[result.passed ? 'success' : 'error'](result.passed ? '身份验证通过' : '身份验证失败，请按提示修复'); invalidate(); setStep(4) },
    onError: (err) => message.error(err instanceof Error ? err.message : '身份验证失败'),
  })
  const submitMutation = useMutation({ mutationFn: () => submitApplicationPublish(selectedId!, onboarding.data?.application.configVersion), onSuccess: () => { message.success('应用已进入待发布状态'); invalidate() }, onError: (err) => message.error(err instanceof Error ? err.message : '提交发布失败') })
  const publishMutation = useMutation({ mutationFn: () => publishApplication(selectedId!, onboarding.data?.application.configVersion), onSuccess: () => { message.success('应用已发布'); invalidate() }, onError: (err) => message.error(err instanceof Error ? err.message : '发布失败') })

  const openConsole = async () => {
    try {
      const result = await getIdentityConsoleLink()
      if (!/^https:\/\//i.test(result.url) && !/^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?(?:\/|$)/i.test(result.url)) throw new Error('身份管理控制台地址未配置')
      window.open(result.url, '_blank', 'noopener,noreferrer')
    } catch (err) { message.error(err instanceof Error ? err.message : '无法打开身份管理控制台') }
  }
  const selectApplication = (id: string) => { setSelectedId(id); setStep(0); form.resetFields() }

  if (!canRead) return <Alert type="warning" showIcon message="没有身份接入查看权限" description="请联系身份管理员开通 iam.integration.read 权限。" />
  if (overviewError) return <QueryErrorState refetch={refetchOverview} />

  return (
    <div>
      <AdminPageHead title="身份与单点登录" extra={overview?.adminEntryEnabled ? <Button icon={<ExportOutlined />} onClick={() => void openConsole()}>打开身份管理控制台</Button> : undefined} />
      <Alert type="info" showIcon style={{ marginBottom: 16 }} message="Velora 管理应用目录与发布；统一身份中心负责用户、MFA 和协议客户端。" description={overview?.adminUrlHost ? `身份提供方：Casdoor · 管理域名：${overview.adminUrlHost}` : '身份控制台入口尚未启用，请联系系统管理员。'} />
      {!overview?.onboardingEnabled ? <Alert type="warning" showIcon style={{ marginBottom: 16 }} message="应用接入向导当前为预留状态" description="开启 VELORA_APPLICATION_ONBOARDING_V2 后可使用身份绑定、验证和发布流程。" /> : null}
      <Card title="选择应用" style={{ marginBottom: 16 }}>
        <Select showSearch optionFilterProp="label" loading={appsLoading} value={selectedId} onChange={selectApplication} placeholder="选择需要配置的应用" style={{ width: '100%', maxWidth: 520 }} options={(apps?.items ?? []).map((app) => ({ value: String(app.id), label: `${app.name}（${app.code}）` }))} />
      </Card>
      {selected && onboarding.data ? <Card title={`${selected.name} · 接入向导`}>
        <Steps current={step} onChange={setStep} items={[{ title: '基础信息' }, { title: '访问方式' }, { title: '身份接入' }, { title: '访问策略' }, { title: '验证与发布' }]} style={{ marginBottom: 28 }} />
        {step === 0 ? <Descriptions bordered column={1} size="small"><Descriptions.Item label="应用编码">{selected.code}</Descriptions.Item><Descriptions.Item label="应用名称">{selected.name}</Descriptions.Item><Descriptions.Item label="负责人">{selected.owner || '-'}</Descriptions.Item><Descriptions.Item label="生命周期"><Tag>{selected.lifecycleStatus || 'PUBLISHED'}</Tag></Descriptions.Item></Descriptions> : null}
        {step === 1 ? <Alert type="info" showIcon message={`当前接入方式：${selected.ssoType === 'URL' ? '普通链接' : selected.ssoType}`} description={selected.ssoType === 'URL' ? '普通链接不需要在身份中心创建客户端。' : 'OIDC 应用必须完成身份绑定和真实 Discovery 验证后才能发布。'} /> : null}
        {step === 2 ? selected.ssoType === 'URL' ? <Alert type="success" showIcon message="普通链接无需身份配置" description="可直接进入验证与发布步骤。" /> : <Form form={form} layout="vertical" initialValues={{ providerApplicationRef: onboarding.data.binding?.providerApplicationRef, publicClientId: onboarding.data.binding?.publicClientId, issuer: onboarding.data.binding?.issuer, redirectUris: onboarding.data.binding?.redirectUris?.join('\n') }} onFinish={(values) => bindingMutation.mutate(values)}><Form.Item label="身份应用引用" name="providerApplicationRef" rules={[{ required: true, message: '请输入身份中心中的应用引用' }]}><Input placeholder="例如：app-portal-demo" /></Form.Item><Form.Item label="公开 Client ID" name="publicClientId" rules={[{ required: true, message: '请输入公开 Client ID' }]}><Input /></Form.Item><Form.Item label="Issuer" name="issuer" rules={[{ required: true, message: '请输入 Issuer' }]}><Input placeholder="https://identity.example.com" /></Form.Item><Form.Item label="Redirect URI（每行一个）" name="redirectUris" rules={[{ required: true, message: '请输入回调地址' }]}><Input.TextArea rows={4} placeholder="https://app.example.com/auth/callback" /></Form.Item><Space><Button type="primary" htmlType="submit" loading={bindingMutation.isPending}>保存身份绑定</Button>{overview?.adminEntryEnabled ? <Button icon={<ExportOutlined />} onClick={() => void openConsole()}>打开身份管理控制台</Button> : null}</Space></Form> : null}
        {step === 3 ? <Alert type="info" showIcon message="配置访问策略" description="请在 Velora 的访问策略页面配置用户、角色、用户组或组织范围；身份中心只负责认证，不替代下游业务授权。" action={<Button size="small" onClick={() => window.location.assign('/admin/policies')}>前往访问策略</Button>} /> : null}
        {step === 4 ? <Space direction="vertical" style={{ width: '100%' }} size="middle"><List size="small" header="最近验证记录" dataSource={onboarding.data.verifications} locale={{ emptyText: '尚无验证记录' }} renderItem={(item) => <List.Item><Space><Tag color={item.result === 'PASSED' ? 'success' : 'error'}>{item.result}</Tag><Text>{item.checkType}</Text><Text type="secondary">{item.errorCode || '通过'}</Text></Space></List.Item>} /><Space wrap>{selected.ssoType !== 'URL' ? <Button icon={<SafetyCertificateOutlined />} loading={verifyMutation.isPending} onClick={() => verifyMutation.mutate()}>执行真实验证</Button> : null}<Button onClick={() => submitMutation.mutate()} loading={submitMutation.isPending} disabled={!onboarding.data.canPublish}>提交发布</Button><Button type="primary" onClick={() => publishMutation.mutate()} loading={publishMutation.isPending} disabled={!onboarding.data.canPublish}>发布应用</Button></Space><Paragraph type="secondary">未完成身份验证的 OIDC 应用不会进入门户。发布操作会记录操作者和配置版本。</Paragraph></Space> : null}
        <Space style={{ marginTop: 24 }}><Button disabled={step === 0} onClick={() => setStep((value) => value - 1)}>上一步</Button><Button disabled={step === 4} onClick={() => setStep((value) => value + 1)}>下一步</Button></Space>
      </Card> : selectedId ? <QueryErrorState refetch={() => void onboarding.refetch()} /> : <Alert type="info" message="请选择应用开始接入" />}
    </div>
  )
}
