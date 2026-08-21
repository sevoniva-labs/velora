import { useEffect, useMemo, useState } from 'react'
import { Alert, App as AntdApp, Button, Card, Descriptions, Empty, Form, Input, List, Modal, Select, Space, Spin, Steps, Tag, Typography } from 'antd'
import { ExportOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AdminPageHead from '../../components/AdminPageHead'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMe } from '../../auth/useMe'
import {
  IDENTITY_CONSOLE,
  IDENTITY_MANAGE,
  IDENTITY_READ,
  IDENTITY_VERIFY,
  PORTAL_PUBLISH,
  hasPermission,
} from '../../auth/permissions'
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

const draftStorageKey = (applicationId: string) => `velora.identity-onboarding.${applicationId}`

type IdentityDraft = {
  providerApplicationRef?: string
  publicClientId?: string
  issuer?: string
  redirectUris?: string
  scopes?: string
  approvalId?: string
}

export default function IdentityIntegrations() {
  usePageTitle('身份与单点登录')
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const me = useMe()
  const [selectedId, setSelectedId] = useState<string>()
  const [step, setStep] = useState(0)
  const [form] = Form.useForm()
  const canRead = hasPermission(me.data?.permissions, IDENTITY_READ, me.data?.roles)
  const canManage = hasPermission(me.data?.permissions, IDENTITY_MANAGE, me.data?.roles)
  const canVerify = hasPermission(me.data?.permissions, IDENTITY_VERIFY, me.data?.roles)
  const canPublish = hasPermission(me.data?.permissions, PORTAL_PUBLISH, me.data?.roles)
  const canOpenConsole = hasPermission(me.data?.permissions, IDENTITY_CONSOLE, me.data?.roles)
  const { data: overview, isError: overviewError, refetch: refetchOverview } = useQuery({ queryKey: queryKeys.identityOverview, queryFn: getIdentityOverview, enabled: Boolean(canRead) })
  const { data: apps, isLoading: appsLoading } = useQuery({ queryKey: queryKeys.adminApplications({ onboarding: true }), queryFn: () => adminListApplications({ page: 1, pageSize: 500 }), enabled: Boolean(canRead) })
  const selected = useMemo(() => apps?.items.find((item) => String(item.id) === selectedId), [apps, selectedId])
  const identityTypeSupported = selected?.ssoType === 'OIDC'
  const onboarding = useQuery({ queryKey: queryKeys.applicationOnboarding(selectedId ?? ''), queryFn: () => getApplicationOnboarding(selectedId!), enabled: Boolean(selectedId) })

  useEffect(() => {
    if (!selectedId || !onboarding.data || selected?.ssoType === 'URL') return
    const serverValues: IdentityDraft = {
      providerApplicationRef: onboarding.data.binding?.providerApplicationRef,
      publicClientId: onboarding.data.binding?.publicClientId,
      issuer: onboarding.data.binding?.issuer,
      redirectUris: onboarding.data.binding?.redirectUris?.join('\n'),
      scopes: onboarding.data.binding?.scopes?.join(' '),
    }
    let restored: IdentityDraft | undefined
    try {
      const raw = window.localStorage.getItem(draftStorageKey(selectedId))
      restored = raw ? JSON.parse(raw) as IdentityDraft : undefined
    } catch {
      restored = undefined
    }
    form.setFieldsValue(restored ?? serverValues)
  }, [form, onboarding.data, selected, selectedId])

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.applicationOnboarding(selectedId ?? '') })
    void queryClient.invalidateQueries({ queryKey: queryKeys.adminApplications() })
    void queryClient.invalidateQueries({ queryKey: queryKeys.applications() })
  }
  const bindingMutation = useMutation({
    mutationFn: (values: { providerApplicationRef: string; publicClientId: string; issuer: string; redirectUris: string; scopes: string; approvalId?: string }) => upsertApplicationIdentityBinding(selectedId!, { providerKey: 'casdoor', protocol: selected?.ssoType ?? 'OIDC', providerApplicationRef: values.providerApplicationRef, publicClientId: values.publicClientId, issuer: values.issuer, redirectUris: values.redirectUris.split(/\r?\n/).map((value) => value.trim()).filter(Boolean), scopes: values.scopes.split(/\s+/).map((value) => value.trim()).filter(Boolean), approvalId: values.approvalId?.trim() || undefined, expectedConfigVersion: onboarding.data?.binding?.configVersion }),
    onSuccess: (result) => {
      const oneTimeClientSecret = result.oneTimeClientSecret
      result.oneTimeClientSecret = undefined
      if (oneTimeClientSecret) {
        Modal.info({ title: 'Client Secret（仅显示一次）', content: <Space direction="vertical" style={{ width: '100%' }}><Alert type="warning" showIcon message="请立即保存到受控 Secret Manager" description="关闭后 Velora 不再提供此值；该值不会写入草稿、审计或浏览器持久化存储。" /><Input.Password value={oneTimeClientSecret} readOnly /></Space>, okText: '已安全保存', maskClosable: false })
      }
      if (selectedId) window.localStorage.removeItem(draftStorageKey(selectedId))
      message.success('身份绑定已保存，等待验证')
      invalidate()
      setStep(3)
    },
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
      <AdminPageHead title="身份与单点登录" extra={overview?.adminEntryEnabled && canOpenConsole ? <Button icon={<ExportOutlined />} onClick={() => void openConsole()}>打开身份管理控制台</Button> : undefined} />
      <Alert type="info" showIcon style={{ marginBottom: 16 }} message="Velora 管理应用目录与发布；统一身份中心负责用户、MFA 和协议客户端。" description={overview?.adminUrlHost ? `身份提供方：Casdoor · 管理域名：${overview.adminUrlHost}${overview.automationEnabled ? ' · 应用自动化已启用' : ''}` : '身份控制台入口尚未启用，请联系系统管理员。'} />
      {overview ? <Descriptions bordered size="small" column={{ xs: 1, sm: 3 }} style={{ marginBottom: 16 }}><Descriptions.Item label="连接状态"><Tag color={overview.connectionStatus === 'CONNECTED' ? 'success' : overview.connectionStatus === 'MISMATCH' ? 'error' : 'warning'}>{overview.connectionStatus === 'CONNECTED' ? '已连接' : overview.connectionStatus === 'MISMATCH' ? 'Issuer 不匹配' : overview.connectionStatus === 'UNCONFIGURED' ? '待配置' : '不可用'}</Tag></Descriptions.Item><Descriptions.Item label="Issuer">{overview.issuer || '-'}</Descriptions.Item><Descriptions.Item label="待处理应用">{overview.pendingApplicationCount}</Descriptions.Item></Descriptions> : null}
      {!overview?.onboardingEnabled ? <Alert type="warning" showIcon style={{ marginBottom: 16 }} message="应用接入向导当前为预留状态" description="开启 VELORA_APPLICATION_ONBOARDING_V2 后可使用身份绑定、验证和发布流程。" /> : null}
      <Card title="选择应用" style={{ marginBottom: 16 }}>
        <Select showSearch optionFilterProp="label" loading={appsLoading} value={selectedId} onChange={selectApplication} placeholder="选择需要配置的应用" style={{ width: '100%', maxWidth: 520 }} options={(apps?.items ?? []).map((app) => ({ value: String(app.id), label: `${app.name}（${app.code}）` }))} notFoundContent={appsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可配置应用" />} />
      </Card>
      {selectedId && onboarding.isLoading ? <Card><Spin tip="正在加载接入配置" /></Card> : null}
      {selectedId && onboarding.isError ? <QueryErrorState refetch={() => void onboarding.refetch()} description="接入配置加载失败，请重试。" /> : null}
      {selected && onboarding.data ? <Card title={`${selected.name} · 接入向导`}>
        <Steps current={step} onChange={setStep} items={[{ title: '基础信息' }, { title: '访问方式' }, { title: '身份接入' }, { title: '访问策略' }, { title: '验证与发布' }]} style={{ marginBottom: 28 }} />
        {step === 0 ? <Descriptions bordered column={1} size="small"><Descriptions.Item label="应用编码">{selected.code}</Descriptions.Item><Descriptions.Item label="应用名称">{selected.name}</Descriptions.Item><Descriptions.Item label="负责人">{selected.owner || '-'}</Descriptions.Item><Descriptions.Item label="生命周期"><Tag>{selected.lifecycleStatus || 'PUBLISHED'}</Tag></Descriptions.Item></Descriptions> : null}
        {step === 1 ? <Alert type={selected.ssoType === 'URL' ? 'info' : identityTypeSupported ? 'info' : 'warning'} showIcon message={`当前接入方式：${selected.ssoType === 'URL' ? '普通链接' : selected.ssoType}`} description={selected.ssoType === 'URL' ? '普通链接不需要在身份中心创建客户端。' : identityTypeSupported ? 'OIDC 应用必须完成身份绑定和真实 Discovery 验证后才能发布。' : '该接入类型暂未验收，当前禁止保存、验证和发布；请迁移为 OIDC 或普通链接。'} /> : null}
        {step === 2 ? selected.ssoType === 'URL' ? <Alert type="success" showIcon message="普通链接无需身份配置" description="可直接进入验证与发布步骤。" /> : !identityTypeSupported ? <Alert type="warning" showIcon message="该接入类型暂未开放" description="SAML、CAS 和 ForwardAuth 在完成独立验收前不能保存或发布。" /> : <Form form={form} layout="vertical" disabled={!canManage} onValuesChange={(_, values: IdentityDraft) => { if (!selectedId) return; try { window.localStorage.setItem(draftStorageKey(selectedId), JSON.stringify(values)) } catch { /* 浏览器存储不可用时仍可继续提交 */ } }} onFinish={(values) => bindingMutation.mutate(values)}><Descriptions bordered column={1} size="small" style={{ marginBottom: 16 }}><Descriptions.Item label="协议">OIDC Authorization Code + PKCE</Descriptions.Item><Descriptions.Item label="Secret">不在 Velora 输入或保存</Descriptions.Item></Descriptions>{!canManage ? <Alert type="info" showIcon style={{ marginBottom: 16 }} message="当前账号仅具备查看权限" description="需要 iam.integration.manage 才能保存身份绑定。" /> : null}<Form.Item label="身份应用引用" name="providerApplicationRef" rules={[{ required: true, message: '请输入身份中心中的应用引用' }]}><Input placeholder="例如：app-portal-demo" /></Form.Item><Form.Item label="公开 Client ID" name="publicClientId" rules={[{ required: true, message: '请输入公开 Client ID' }]}><Input /></Form.Item><Form.Item label="Issuer" name="issuer" rules={[{ required: true, message: '请输入 Issuer' }]}><Input placeholder="https://identity.example.com" /></Form.Item><Form.Item label="Scopes" name="scopes" initialValue="openid profile email" rules={[{ required: true, message: '请输入 OIDC Scopes' }]}><Input placeholder="openid profile email" /></Form.Item><Form.Item label="Redirect URI（每行一个）" name="redirectUris" rules={[{ required: true, message: '请输入回调地址' }]}><Input.TextArea rows={4} placeholder="https://app.example.com/auth/callback" /></Form.Item>{overview?.automationEnabled ? <Form.Item label="自动化审批执行票据" name="approvalId" rules={[{ required: true, message: '自动化开启时请输入已批准的执行票据' }]}><Input placeholder="审批中心生成的 approval_id" /></Form.Item> : null}<Space><Button type="primary" htmlType="submit" loading={bindingMutation.isPending} disabled={!canManage}>保存身份绑定</Button>{overview?.adminEntryEnabled && canOpenConsole ? <Button icon={<ExportOutlined />} onClick={() => void openConsole()}>打开身份管理控制台</Button> : null}</Space></Form> : null}
        {step === 3 ? <Alert type="info" showIcon message="配置访问策略" description="请在 Velora 的访问策略页面配置用户、角色、用户组或组织范围；身份中心只负责认证，不替代下游业务授权。" action={canManage ? <Button size="small" onClick={() => window.location.assign('/admin/policies')}>前往访问策略</Button> : undefined} /> : null}
        {step === 4 ? <Space direction="vertical" style={{ width: '100%' }} size="middle"><List size="small" header="最近验证记录" dataSource={onboarding.data.verifications} locale={{ emptyText: '尚无验证记录' }} renderItem={(item) => <List.Item><Space><Tag color={item.result === 'PASSED' ? 'success' : 'error'}>{item.result}</Tag><Text>{item.checkType}</Text><Text type="secondary">{item.errorCode || '通过'}</Text></Space></List.Item>} /><Space wrap>{identityTypeSupported ? <Button icon={<SafetyCertificateOutlined />} loading={verifyMutation.isPending} onClick={() => verifyMutation.mutate()} disabled={!canVerify}>执行真实验证</Button> : null}<Button onClick={() => submitMutation.mutate()} loading={submitMutation.isPending} disabled={!canPublish || !onboarding.data.canPublish}>提交发布</Button><Button type="primary" onClick={() => publishMutation.mutate()} loading={publishMutation.isPending} disabled={!canPublish || !onboarding.data.canPublish}>发布应用</Button></Space><Paragraph type="secondary">未完成身份验证的 OIDC 应用不会进入门户。发布操作会记录操作者和配置版本。</Paragraph></Space> : null}
        <Space style={{ marginTop: 24 }}><Button disabled={step === 0} onClick={() => setStep((value) => value - 1)}>上一步</Button><Button disabled={step === 4} onClick={() => setStep((value) => value + 1)}>下一步</Button></Space>
      </Card> : !selectedId && !appsLoading && (apps?.items?.length ?? 0) === 0 ? <Empty description="暂无可配置应用" /> : !selectedId ? <Alert type="info" message="请选择应用开始接入" /> : null}
    </div>
  )
}
