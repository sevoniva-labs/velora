import { useMemo, useState } from 'react'
import { App, Button, Modal, Popconfirm, Tag, Typography } from 'antd'
import { KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProFormDigit, ProFormSelect, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createIntegrationToken, listIntegrationTokens, revokeIntegrationToken, type IntegrationToken } from '../../api/api'
import { useMe } from '../../auth/useMe'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'
import { useClientTableSearch } from '../../utils/tableSearch'
import QueryErrorState from '../../components/QueryErrorState'

interface CreateForm { name: string; scopes: string[]; expiresInDays: number }

const PERMISSION_OBJECTS: Record<string, string> = { application: '应用', integration: '统一登录', console: '身份管理入口', audit: '操作记录', api_token: '接口凭据', user: '用户', assignment: '任职', department: '部门', position: '岗位', user_group: '用户组', role: '平台角色', session: '登录会话', temporary_grant: '临时权限', access_review: '权限复核', request: '审批申请', task: '审批任务', config: '平台配置', security: '登录安全' }
const PERMISSION_ACTIONS: Record<string, string> = { read: '查看', create: '新建', update: '编辑', manage: '管理', publish: '上线', verify: '检查', open: '打开', revoke: '退出', decide: '审批' }
function permissionLabel(permission: string): string {
  if (permission === '*') return '全部权限'
  const parts = permission.split('.')
  const action = parts.pop() ?? ''
  const object = parts.pop() ?? ''
  return `${PERMISSION_OBJECTS[object] ?? object} · ${PERMISSION_ACTIONS[action] ?? action}`
}

export default function AdminIntegrationTokens() {
  usePageTitle('接口凭据')
  const { message } = App.useApp()
  const me = useMe()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [secret, setSecret] = useState<string>()
  const tokens = useQuery({ queryKey: ['admin', 'integration-tokens'], queryFn: listIntegrationTokens })
  const tokenTable = useClientTableSearch(tokens.data ?? [], { exact: ['revoked'] })
  const scopeOptions = useMemo(() => {
    const permissions = (me.data?.permissions ?? []).filter((item) => item !== '*').sort()
    const options = permissions.map((value) => ({ value, label: permissionLabel(value) }))
    return me.data?.roles?.includes('system_admin') ? [{ value: '*', label: '全部权限' }, ...options] : options
  }, [me.data?.permissions, me.data?.roles])
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'integration-tokens'] })
  const create = useMutation({ mutationFn: createIntegrationToken, onSuccess: async (result) => { setSecret(result.token); setOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '对接账号创建失败') })
  const revoke = useMutation({ mutationFn: revokeIntegrationToken, onSuccess: async () => { message.success('对接账号已停用'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '对接账号停用失败') })
  const columns: ProColumns<IntegrationToken>[] = [
    { title: '名称', dataIndex: 'name' },
    { title: '允许操作', dataIndex: 'scopes', search: false, render: (_, row) => row.scopes.map((scope) => <Tag key={scope}>{permissionLabel(scope)}</Tag>) },
    { title: '过期时间', dataIndex: 'expiresAt', valueType: 'dateTime', search: false, render: (_, row) => row.expiresAt ? formatDateTime(row.expiresAt) : '永不过期' },
    { title: '最近使用', dataIndex: 'lastUsedAt', valueType: 'dateTime', search: false, render: (_, row) => row.lastUsedAt ? formatDateTime(row.lastUsedAt) : '未使用' },
    { title: '状态', dataIndex: 'revoked', valueType: 'select', valueEnum: { false: { text: '可用' }, true: { text: '已停用' } }, render: (_, row) => row.revoked ? <Tag>已停用</Tag> : <Tag color="success">可用</Tag> },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => row.revoked ? null : <Popconfirm title="停用此对接账号？" description="停用后，使用该密钥的系统将无法访问平台。" okText="停用" okButtonProps={{ danger: true }} onConfirm={() => revoke.mutate(row.id)}><Button type="link" danger>停用</Button></Popconfirm> },
  ]
  return <PageContainer title="接口凭据">
    {tokens.isError ? <QueryErrorState refetch={tokens.refetch} /> : <ProTable<IntegrationToken> rowKey="id" columns={columns} {...tokenTable} loading={tokens.isLoading} search={{ labelWidth: 'auto' }} pagination={false} toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>新建对接账号</Button>]} />}
    <ModalForm<CreateForm> title="新建对接账号" open={open} onOpenChange={setOpen} initialValues={{ expiresInDays: 90 }} submitter={{ searchConfig: { submitText: '创建', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
      <ProFormText name="name" label="名称" fieldProps={{ prefix: <KeyOutlined />, maxLength: 100 }} rules={[{ required: true, message: '请输入名称' }]} />
      <ProFormSelect name="scopes" label="允许操作" mode="multiple" options={scopeOptions} rules={[{ required: true, message: '请选择允许执行的操作' }]} />
      <ProFormDigit name="expiresInDays" label="有效期（天）" min={1} max={365} rules={[{ required: true, message: '请输入有效期' }]} />
    </ModalForm>
    <Modal title="保存对接密钥" open={Boolean(secret)} onCancel={() => setSecret(undefined)} onOk={() => setSecret(undefined)} cancelButtonProps={{ style: { display: 'none' } }} okText="已保存" maskClosable={false} closable={false}>
      <Typography.Paragraph type="secondary">该密钥仅显示一次，请立即保存。</Typography.Paragraph>
      <Typography.Text code copyable>{secret}</Typography.Text>
    </Modal>
  </PageContainer>
}
