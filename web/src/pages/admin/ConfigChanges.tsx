import { useMemo, useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProForm, ProFormCheckbox, ProFormSelect, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createApproval, createConfigChange, listApprovals, listConfigChanges, transitionConfigChange } from '../../api/admin-platform'
import type { ApprovalRequest, ConfigChange } from '../../types'
import { useMe } from '../../auth/useMe'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'
import AdminUserSelect from '../../components/AdminUserSelect'
import { SYSTEM_CONFIG_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import QueryErrorState from '../../components/QueryErrorState'
import { AdminListScope, AdminListSearch } from '../../components/admin/AdminListToolbar'

interface CreateForm { namespace: string; group: string; dataId: string; valueDigest: string; valueRef: string; sensitive: boolean }
interface ApprovalForm { approverId: string }
type ChangeAction = 'APPROVE' | 'PUBLISH' | 'REQUEST_ROLLBACK' | 'ROLLBACK'
const STATE_LABELS: Record<string, string> = { PENDING_APPROVAL: '待审批', APPROVED: '待生效', PUBLISHED: '已生效', ROLLBACK_PENDING: '待恢复', ROLLED_BACK: '已恢复', REJECTED: '已拒绝' }
const ACTIONS: Record<string, { action: ChangeAction; label: string; requestType: string; auditAction: string }> = {
  PENDING_APPROVAL: { action: 'APPROVE', label: '提交审批', requestType: 'CONFIG_CHANGE_APPROVE', auditAction: 'config_change.approve' },
  APPROVED: { action: 'PUBLISH', label: '申请生效', requestType: 'CONFIG_CHANGE_PUBLISH', auditAction: 'config_change.publish' },
  PUBLISHED: { action: 'REQUEST_ROLLBACK', label: '申请恢复上一版本', requestType: 'CONFIG_CHANGE_ROLLBACK_REQUEST', auditAction: 'config_change.rollback.request' },
  ROLLBACK_PENDING: { action: 'ROLLBACK', label: '恢复上一版本', requestType: 'CONFIG_CHANGE_ROLLBACK', auditAction: 'config_change.rollback' },
}

export default function ConfigChanges() {
  usePageTitle('配置发布')
  const { message } = App.useApp()
  const me = useMe()
  const canManage = useAdminPermission(SYSTEM_CONFIG_MANAGE)
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [requesting, setRequesting] = useState<ConfigChange>()
  const [scope, setScope] = useState('ALL')
  const [searchValue, setSearchValue] = useState('')
  const [keyword, setKeyword] = useState('')
  const changes = useQuery({ queryKey: ['admin', 'config-changes'], queryFn: listConfigChanges })
  const allChanges = changes.data ?? []
  const visibleChanges = allChanges.filter((item) => (scope === 'ALL' || (scope === 'ACTIVE' ? ['PENDING_APPROVAL', 'APPROVED', 'ROLLBACK_PENDING'].includes(item.state) : scope === 'ENDED' ? ['ROLLED_BACK', 'REJECTED'].includes(item.state) : item.state === scope)) && (!keyword || `${item.dataId} ${item.namespace} ${item.group}`.toLowerCase().includes(keyword.toLowerCase())))
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canManage })
  const approvedByResource = useMemo(() => new Map((approvals.data ?? []).filter((item) => item.resource === 'config_change' && item.status === 'APPROVED').map((item) => [`${item.resourceId}:${item.action}`, item])), [approvals.data])
  const createInput = (values: CreateForm) => {
    const namespace = values.namespace.trim()
    const group = values.group.trim()
    const dataId = values.dataId.trim()
    return { ...values, namespace, group, dataId, expectedPreviousVersion: 0, version: 0, valueDigest: values.valueDigest.trim().toLowerCase(), valueRef: values.valueRef.trim() }
  }
  const refresh = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'config-changes'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] })]) }
  const create = useMutation({ mutationFn: createConfigChange, onSuccess: async () => { message.success('配置发布单已创建'); setCreateOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '配置发布单创建失败') })
  const request = useMutation({ mutationFn: (values: ApprovalForm) => { const config = ACTIONS[requesting!.state]; return createApproval({ requestType: config.requestType, action: config.auditAction, resource: 'config_change', resourceId: requesting!.id, summary: `${config.label}：${requesting!.dataId} v${requesting!.version}`, payloadJson: JSON.stringify({ change_id: requesting!.id, action: config.action }), approverIds: [values.approverId] }) }, onSuccess: async () => { message.success('申请已提交'); setRequesting(undefined); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '申请提交失败') })
  const execute = useMutation({ mutationFn: ({ change, approval }: { change: ConfigChange; approval: ApprovalRequest }) => transitionConfigChange(change.id, ACTIONS[change.state].action, approval.id), onSuccess: async () => { message.success('配置状态已更新'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '操作失败') })
  const columns: ProColumns<ConfigChange>[] = [
    { title: '配置项', dataIndex: 'dataId', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.dataId}</Typography.Text><Typography.Text type="secondary">{row.namespace} / {row.group}</Typography.Text></Space> },
    { title: '版本', dataIndex: 'version', width: 90, render: (_, row) => `v${row.version}` },
    { title: '配置文件', dataIndex: 'valueRef', search: false, ellipsis: true },
    { title: '更新时间', dataIndex: 'updatedAt', valueType: 'dateTime', search: false, render: (_, row) => formatDateTime(row.updatedAt) },
    { title: '状态', dataIndex: 'state', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(STATE_LABELS).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag color={row.state === 'PUBLISHED' ? 'success' : row.state.includes('PENDING') || row.state === 'APPROVED' ? 'processing' : 'default'}>{STATE_LABELS[row.state] ?? '已结束'}</Tag> },
  ]
  if (canManage) columns.push({ title: '操作', valueType: 'option', width: 120, render: (_, row) => { const config = ACTIONS[row.state]; if (!config) return <Typography.Text type="secondary">—</Typography.Text>; const approved = approvedByResource.get(`${row.id}:${config.auditAction}`); return approved ? <Button type="link" loading={execute.isPending} onClick={() => execute.mutate({ change: row, approval: approved })}>执行</Button> : <Button type="link" onClick={() => setRequesting(row)}>{config.label}</Button> } })
  return <PageContainer title="配置发布">
    {changes.isError ? <QueryErrorState refetch={changes.refetch} /> : <ProTable<ConfigChange> className="velora-admin-primary-table" rowKey="id" columns={columns} dataSource={visibleChanges} loading={changes.isLoading} search={false} pagination={{ pageSize: 20 }} headerTitle={<AdminListScope value={scope} onChange={setScope} options={[{ label: '全部变更', value: 'ALL', count: allChanges.length }, { label: '处理中', value: 'ACTIVE', count: allChanges.filter((item) => ['PENDING_APPROVAL', 'APPROVED', 'ROLLBACK_PENDING'].includes(item.state)).length }, { label: '已生效', value: 'PUBLISHED', count: allChanges.filter((item) => item.state === 'PUBLISHED').length }, { label: '已结束', value: 'ENDED', count: allChanges.filter((item) => ['ROLLED_BACK', 'REJECTED'].includes(item.state)).length }]} />} toolBarRender={() => [<AdminListSearch key="tools" value={searchValue} placeholder="搜索变更名称或环境" onChange={setSearchValue} onSearch={setKeyword}>{canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>新建变更</Button>}</AdminListSearch>]} />}
    <ModalForm<CreateForm> title="新建配置发布单" open={createOpen} onOpenChange={setCreateOpen} width={640} modalProps={{ centered: true }} initialValues={{ sensitive: false }} submitter={{ searchConfig: { submitText: '创建发布单', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(createInput(values)); return true }}>
      <ProFormSelect name="namespace" label="发布环境" width="md" options={[{ label: '生产环境', value: 'production' }, { label: '测试环境', value: 'testing' }, { label: '开发环境', value: 'development' }]} rules={[{ required: true, message: '请选择发布环境' }]} />
      <ProFormSelect name="group" label="配置领域" width="md" options={[{ label: '门户与应用', value: 'portal' }, { label: '组织与权限', value: 'identity' }, { label: '登录与安全', value: 'security' }, { label: '系统集成', value: 'integration' }]} rules={[{ required: true, message: '请选择配置领域' }]} />
      <ProFormText name="dataId" label="变更名称" width="md" fieldProps={{ maxLength: 100 }} rules={[{ required: true, message: '请输入便于识别的变更名称' }]} />
      <ProFormText name="valueRef" label="配置文件" tooltip="填写已上传到受控存储的文件地址" width="lg" rules={[{ required: true, message: '请输入受控存储中的配置文件地址' }]} />
      <ProFormText name="valueDigest" label="文件校验值" tooltip="用于确认发布文件未被篡改，由配置文件交付流程生成" width="lg" rules={[{ required: true, pattern: /^[a-fA-F0-9]{64}$/, message: '请输入 64 位 SHA-256 校验值' }]} />
      <ProFormCheckbox name="sensitive">文件包含密码或密钥</ProFormCheckbox>
    </ModalForm>
    <ModalForm<ApprovalForm> key={requesting?.id ?? 'approval'} title={requesting ? ACTIONS[requesting.state]?.label : '提交申请'} open={Boolean(requesting)} onOpenChange={(value) => !value && setRequesting(undefined)} submitter={{ searchConfig: { submitText: '提交申请', resetText: '取消' } }} onFinish={async (values) => { await request.mutateAsync(values); return true }}>
      <ProForm.Item name="approverId" label="审批人" rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
    </ModalForm>
  </PageContainer>
}
