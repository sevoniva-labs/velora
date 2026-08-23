import { useEffect, useMemo, useState } from 'react'
import { Alert, App as AntdApp, Button, Card, Descriptions, Divider, Empty, Form, Input, List, Modal, Select, Space, Spin, Steps, Tag, Typography } from 'antd'
import { DeleteOutlined, ExportOutlined, PlusOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
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
  adminGetApplicationProvisioningTarget,
  adminListApplicationRoles,
  adminReplaceApplicationRoles,
  adminUpsertApplicationProvisioningTarget,
  getApplicationOnboarding,
  getIdentityConsoleLink,
  getIdentityOverview,
  publishApplication,
  queryKeys,
  submitApplicationPublish,
  upsertApplicationIdentityBinding,
  verifyApplicationIdentity,
  type ApplicationRole,
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
  const [searchParams, setSearchParams] = useSearchParams()
  const me = useMe()
  const [selectedId, setSelectedId] = useState<string | undefined>(() => searchParams.get('application') || undefined)
  const [step, setStep] = useState(0)
  const [form] = Form.useForm()
  const [accessForm] = Form.useForm<{ endpointUrl: string; roles: Array<Pick<ApplicationRole, 'roleKey' | 'name' | 'description' | 'riskLevel' | 'status'>> }>()
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
  const rolesQuery = useQuery({ queryKey: queryKeys.applicationRoles(selectedId ?? ''), queryFn: () => adminListApplicationRoles(selectedId!), enabled: Boolean(selectedId) })
  const targetQuery = useQuery({ queryKey: queryKeys.applicationProvisioningTarget(selectedId ?? ''), queryFn: () => adminGetApplicationProvisioningTarget(selectedId!), enabled: Boolean(selectedId) })

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

  useEffect(() => {
    if (!selectedId || rolesQuery.isLoading || targetQuery.isLoading) return
    accessForm.setFieldsValue({
      endpointUrl: targetQuery.data?.endpointUrl ?? '',
      roles: (rolesQuery.data ?? []).map(({ roleKey, name, description, riskLevel, status }) => ({ roleKey, name, description, riskLevel, status })),
    })
  }, [accessForm, rolesQuery.data, rolesQuery.isLoading, selectedId, targetQuery.data, targetQuery.isLoading])

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.applicationOnboarding(selectedId ?? '') })
    void queryClient.invalidateQueries({ queryKey: queryKeys.adminApplications() })
    void queryClient.invalidateQueries({ queryKey: queryKeys.applications() })
    if (selectedId) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.applicationRoles(selectedId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.applicationProvisioningTarget(selectedId) })
    }
  }
  const accessMutation = useMutation({
    mutationFn: async (values: { endpointUrl: string; roles: Array<Pick<ApplicationRole, 'roleKey' | 'name' | 'description' | 'riskLevel' | 'status'>> }) => {
      const roles = await adminReplaceApplicationRoles(selectedId!, values.roles ?? [])
      const target = await adminUpsertApplicationProvisioningTarget(selectedId!, { endpointUrl: values.endpointUrl, expectedConfigVersion: targetQuery.data?.configVersion })
      return { roles, ...target }
    },
    onSuccess: (result) => {
      const secret = result.oneTimeProvisioningSecret
      if (secret) {
        Modal.info({ title: '账号同步密钥（仅显示一次）', content: <Space direction="vertical" style={{ width: '100%' }}><Alert type="warning" showIcon message="请立即写入目标应用的 Secret Manager" description="关闭后无法再次查看；Velora 只保存信封加密密文。遗失时请执行密钥轮换。" /><Input.Password value={secret} readOnly visibilityToggle /></Space>, okText: '已安全保存', maskClosable: false })
      }
      message.success('账号与权限配置已保存')
      invalidate()
      setStep(3)
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '账号与权限配置保存失败'),
  })
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
  const selectApplication = (id: string) => { setSelectedId(id); setSearchParams({ application: id }, { replace: true }); setStep(0); form.resetFields(); accessForm.resetFields() }

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
        <Alert type={onboarding.data.blockers.length ? 'warning' : onboarding.data.onboardingStatus === 'PUBLISHED' ? 'success' : 'info'} showIcon style={{ marginBottom: 20 }} message={<Space><span>{onboarding.data.onboardingStatus}</span><span>{onboarding.data.nextAction}</span></Space>} description={onboarding.data.blockers.length ? `阻塞项：${onboarding.data.blockers.join('；')}` : '当前配置没有阻塞项。'} />
        <Steps current={step} onChange={setStep} items={[{ title: '应用资料' }, { title: '统一登录' }, { title: '账号与权限' }, { title: '接入配置' }, { title: '验证与发布' }]} style={{ marginBottom: 28 }} />
        {step === 0 ? <Descriptions bordered column={1} size="small"><Descriptions.Item label="应用编码">{selected.code}</Descriptions.Item><Descriptions.Item label="应用名称">{selected.name}</Descriptions.Item><Descriptions.Item label="负责人">{selected.owner || '-'}</Descriptions.Item><Descriptions.Item label="生命周期"><Tag>{selected.lifecycleStatus || 'PUBLISHED'}</Tag></Descriptions.Item></Descriptions> : null}
        {step === 1 ? <Alert type={selected.ssoType === 'URL' ? 'info' : identityTypeSupported ? 'info' : 'warning'} showIcon message={`当前接入方式：${selected.ssoType === 'URL' ? '普通链接' : selected.ssoType}`} description={selected.ssoType === 'URL' ? '普通链接不需要在身份中心创建客户端。' : identityTypeSupported ? 'OIDC 应用必须完成身份绑定和真实 Discovery 验证后才能发布。' : '该接入类型暂未验收，当前禁止保存、验证和发布；请迁移为 OIDC 或普通链接。'} /> : null}
        {step === 2 ? <Form form={accessForm} layout="vertical" disabled={!canManage} onFinish={(values) => accessMutation.mutate(values)} initialValues={{ roles: [] }}>
          <Alert type="info" showIcon message="账号由 Velora 统一下发，业务权限由目标应用执行" description="角色编码一旦被目标应用使用应保持稳定。未配置访问策略时默认没有用户可见；确需全员开放时请显式选择全体成员。" style={{ marginBottom: 16 }} />
          <Form.Item label="账号同步地址" name="endpointUrl" rules={[{ required: true, message: '请输入账号同步地址' }, { type: 'url', message: '请输入完整 HTTPS 地址' }, { pattern: /^https:\/\//i, message: '生产接入只允许 HTTPS' }]} extra="例如 https://app.example.com/api/v1/integrations/velora/provisioning">
            <Input placeholder="https://app.example.com/api/v1/integrations/velora/provisioning" />
          </Form.Item>
          {targetQuery.data ? <Descriptions bordered size="small" column={{ xs: 1, sm: 3 }} style={{ marginBottom: 16 }}><Descriptions.Item label="同步状态"><Tag>{targetQuery.data.deliveryStatus}</Tag></Descriptions.Item><Descriptions.Item label="密钥版本">v{targetQuery.data.activeKeyVersion}</Descriptions.Item><Descriptions.Item label="密钥指纹">{targetQuery.data.secretFingerprint || '-'}</Descriptions.Item></Descriptions> : null}
          <Divider>应用角色</Divider>
          <Form.List name="roles">
            {(fields, { add, remove }) => <Space direction="vertical" style={{ width: '100%' }} size="middle">
              {fields.map(({ key, name, ...rest }) => <Card key={key} size="small">
                <div className="velora-form-grid">
                  <Form.Item {...rest} label="角色编码" name={[name, 'roleKey']} rules={[{ required: true, message: '请输入角色编码' }, { pattern: /^[a-z][a-z0-9._-]{0,99}$/, message: '使用小写字母开头，可包含数字 . _ -' }]}><Input placeholder="developer" /></Form.Item>
                  <Form.Item {...rest} label="显示名称" name={[name, 'name']} rules={[{ required: true, message: '请输入显示名称' }]}><Input placeholder="开发人员" /></Form.Item>
                </div>
                <div className="velora-form-grid">
                  <Form.Item {...rest} label="风险级别" name={[name, 'riskLevel']} initialValue="NORMAL"><Select options={[{ value: 'NORMAL', label: '普通' }, { value: 'PRIVILEGED', label: '高权限' }, { value: 'CRITICAL', label: '关键权限' }]} /></Form.Item>
                  <Form.Item {...rest} label="状态" name={[name, 'status']} initialValue="ACTIVE"><Select options={[{ value: 'ACTIVE', label: '启用' }, { value: 'DISABLED', label: '停用' }]} /></Form.Item>
                </div>
                <Form.Item {...rest} label="说明" name={[name, 'description']}><Input placeholder="说明该角色可执行的业务操作" /></Form.Item>
                <Button danger type="text" icon={<DeleteOutlined />} onClick={() => remove(name)}>移除角色</Button>
              </Card>)}
              <Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ riskLevel: 'NORMAL', status: 'ACTIVE' })}>添加角色</Button>
            </Space>}
          </Form.List>
          <Space wrap style={{ marginTop: 20 }}><Button type="primary" htmlType="submit" loading={accessMutation.isPending}>保存账号与权限配置</Button><Button onClick={() => window.location.assign('/admin/policies')}>配置访问范围</Button>{targetQuery.data ? <Button danger loading={accessMutation.isPending} onClick={() => accessForm.validateFields().then((values) => adminUpsertApplicationProvisioningTarget(selectedId!, { endpointUrl: values.endpointUrl, rotateSecret: true, expectedConfigVersion: targetQuery.data?.configVersion })).then((result) => { if (result.oneTimeProvisioningSecret) Modal.info({ title: '新账号同步密钥（仅显示一次）', content: <Input.Password value={result.oneTimeProvisioningSecret} readOnly visibilityToggle />, okText: '已安全保存', maskClosable: false }); invalidate(); message.success('密钥已轮换') }).catch((err) => message.error(err instanceof Error ? err.message : '密钥轮换失败'))}>轮换同步密钥</Button> : null}</Space>
        </Form> : null}
        {step === 3 ? selected.ssoType === 'URL' ? <Alert type="success" showIcon message="普通链接无需身份配置" description="账号与访问策略保存后可直接进入验证与发布。" /> : !identityTypeSupported ? <Alert type="warning" showIcon message="该接入类型暂未开放" description="当前仅开放 OIDC 与普通链接。" /> : <Form form={form} layout="vertical" disabled={!canManage} onValuesChange={(_, values: IdentityDraft) => { if (!selectedId) return; try { window.localStorage.setItem(draftStorageKey(selectedId), JSON.stringify(values)) } catch { /* 仅保存非敏感草稿 */ } }} onFinish={(values) => bindingMutation.mutate(values)}><Descriptions bordered column={1} size="small" style={{ marginBottom: 16 }}><Descriptions.Item label="登录协议">OIDC Authorization Code + PKCE S256</Descriptions.Item><Descriptions.Item label="账号入口">用户始终从 Velora 登录，Casdoor 不直接暴露</Descriptions.Item><Descriptions.Item label="凭据保护">Client Secret 仅签发时显示一次</Descriptions.Item></Descriptions>{!canManage ? <Alert type="info" showIcon style={{ marginBottom: 16 }} message="当前账号仅具备查看权限" /> : null}<Form.Item label="回调地址（每行一个）" name="redirectUris" rules={[{ required: true, message: '请输入回调地址' }]}><Input.TextArea rows={3} placeholder="https://app.example.com/api/v1/auth/oidc/callback" /></Form.Item><details><summary style={{ cursor: 'pointer', marginBottom: 16 }}>高级设置</summary><Form.Item label="身份应用引用" name="providerApplicationRef" rules={[{ required: true, message: '请输入身份应用引用' }]}><Input /></Form.Item><Form.Item label="公开 Client ID" name="publicClientId" rules={[{ required: true, message: '请输入 Client ID' }]}><Input /></Form.Item><Form.Item label="Issuer" name="issuer" rules={[{ required: true, message: '请输入 Issuer' }]}><Input /></Form.Item><Form.Item label="Scopes" name="scopes" initialValue="openid profile email"><Input readOnly={Boolean(overview?.automationEnabled)} /></Form.Item>{overview?.automationEnabled ? <Form.Item label="审批执行票据" name="approvalId" rules={[{ required: true, message: '请输入已批准的执行票据' }]}><Input /></Form.Item> : null}</details><Button type="primary" htmlType="submit" loading={bindingMutation.isPending}>生成并保存接入配置</Button></Form> : null}
        {step === 4 ? <Space direction="vertical" style={{ width: '100%' }} size="middle"><List size="small" header="最近验证记录" dataSource={onboarding.data.verifications} locale={{ emptyText: '尚无验证记录' }} renderItem={(item) => <List.Item><Space><Tag color={item.result === 'PASSED' ? 'success' : 'error'}>{item.result}</Tag><Text>{item.checkType}</Text><Text type="secondary">{item.errorCode || '通过'}</Text></Space></List.Item>} /><Space wrap>{identityTypeSupported ? <Button icon={<SafetyCertificateOutlined />} loading={verifyMutation.isPending} onClick={() => verifyMutation.mutate()} disabled={!canVerify}>执行真实验证</Button> : null}<Button onClick={() => submitMutation.mutate()} loading={submitMutation.isPending} disabled={!canPublish || !onboarding.data.canPublish}>提交发布</Button><Button type="primary" onClick={() => publishMutation.mutate()} loading={publishMutation.isPending} disabled={!canPublish || !onboarding.data.canPublish}>发布应用</Button></Space><Paragraph type="secondary">未完成身份验证的 OIDC 应用不会进入门户。发布操作会记录操作者和配置版本。</Paragraph></Space> : null}
        <Space style={{ marginTop: 24 }}><Button disabled={step === 0} onClick={() => setStep((value) => value - 1)}>上一步</Button><Button disabled={step === 4} onClick={() => setStep((value) => value + 1)}>下一步</Button></Space>
      </Card> : !selectedId && !appsLoading && (apps?.items?.length ?? 0) === 0 ? <Empty description="暂无可配置应用" /> : !selectedId ? <Alert type="info" message="请选择应用开始接入" /> : null}
    </div>
  )
}
