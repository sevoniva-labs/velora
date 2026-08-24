import { useMemo, useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProForm, ProFormDateTimePicker, ProFormSelect, ProFormTextArea, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs, { type Dayjs } from 'dayjs'
import { createApproval, createTemporaryRoleGrant, listApprovals, listPlatformRoles, listTemporaryRoleGrants, revokeTemporaryRoleGrant } from '../../api/admin-platform'
import type { ApprovalRequest, TemporaryRoleGrant } from '../../types'
import { useMe } from '../../auth/useMe'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'
import AdminUserSelect from '../../components/AdminUserSelect'
import { SYSTEM_TEMPORARY_GRANT_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import { useClientTableSearch } from '../../utils/tableSearch'

interface RequestForm { userId: string; roleKey: string; reason: string; validUntil: Dayjs; approverId: string }
interface RevokeForm { reason: string }
const STATUS_LABELS: Record<string, string> = { SCHEDULED: '待生效', ACTIVE: '生效中', EXPIRED: '已过期', REVOKED: '已撤销' }

export default function TemporaryGrants() {
  usePageTitle('临时授权')
  const { message } = App.useApp()
  const me = useMe()
  const canManage = useAdminPermission(SYSTEM_TEMPORARY_GRANT_MANAGE)
  const queryClient = useQueryClient()
  const [requestOpen, setRequestOpen] = useState(false)
  const [revoking, setRevoking] = useState<TemporaryRoleGrant>()
  const [targetUserName, setTargetUserName] = useState('')
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const grants = useQuery({ queryKey: ['admin', 'temporary-grants'], queryFn: listTemporaryRoleGrants })
  const grantTable = useClientTableSearch(grants.data ?? [], { exact: ['status'] })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canManage })
  const roleNames = useMemo(() => new Map((roles.data ?? []).map((item) => [item.key, item.name])), [roles.data])
  const executed = new Set((grants.data ?? []).map((item) => item.approvalId))
  const approved = (approvals.data ?? []).filter((item) => item.requestType === 'TEMPORARY_ROLE_GRANT' && item.status === 'APPROVED' && !executed.has(item.id))
  const refresh = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'temporary-grants'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] })]) }
  const request = useMutation({ mutationFn: async (values: RequestForm) => { const validFrom = new Date().toISOString(); const validUntil = values.validUntil.toISOString(); const payload = { reason: values.reason, role_key: values.roleKey, user_id: values.userId, valid_from: validFrom, valid_until: validUntil }; return createApproval({ requestType: 'TEMPORARY_ROLE_GRANT', action: 'temporary_role_grant.create', resource: 'user', resourceId: values.userId, summary: `临时授予${targetUserName || '用户'}“${roleNames.get(values.roleKey) ?? values.roleKey}”角色`, payloadJson: JSON.stringify(payload), approverIds: [values.approverId] }) }, onSuccess: async () => { message.success('临时授权申请已提交'); setRequestOpen(false); setTargetUserName(''); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '申请提交失败') })
  const execute = useMutation({ mutationFn: (approval: ApprovalRequest) => { const payload = JSON.parse(approval.payloadJson) as { user_id: string; role_key: string; reason: string; valid_from: string; valid_until: string }; return createTemporaryRoleGrant({ userId: payload.user_id, roleKey: payload.role_key, reason: payload.reason, validFrom: payload.valid_from, validUntil: payload.valid_until, approvalId: approval.id }) }, onSuccess: async () => { message.success('临时授权已生效'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '临时授权执行失败') })
  const revoke = useMutation({ mutationFn: (values: RevokeForm) => revokeTemporaryRoleGrant(revoking!.id, values.reason), onSuccess: async () => { message.success('临时授权已撤销'); setRevoking(undefined); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '撤销失败') })
  const columns: ProColumns<TemporaryRoleGrant>[] = [
    { title: '用户', dataIndex: 'loginName', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text>{row.displayName || row.loginName}</Typography.Text>{row.displayName && <Typography.Text type="secondary">{row.loginName}</Typography.Text>}</Space> },
    { title: '平台角色', dataIndex: 'roleKey', render: (_, row) => roleNames.get(row.roleKey) ?? row.roleKey },
    { title: '有效期', search: false, render: (_, row) => `${formatDateTime(row.validFrom)} 至 ${formatDateTime(row.validUntil)}` },
    { title: '原因', dataIndex: 'reason', search: false, ellipsis: true },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(STATUS_LABELS).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag color={row.status === 'ACTIVE' ? 'success' : 'default'}>{STATUS_LABELS[row.status] ?? '已结束'}</Tag> },
  ]
  if (canManage) columns.push({ title: '操作', valueType: 'option', width: 90, render: (_, row) => row.status === 'ACTIVE' || row.status === 'SCHEDULED' ? <Button type="link" danger onClick={() => setRevoking(row)}>撤销</Button> : <Typography.Text type="secondary">—</Typography.Text> })
  const approvedColumns: ProColumns<ApprovalRequest>[] = [
    { title: '已批准事项', dataIndex: 'summary' },
    { title: '批准时间', dataIndex: 'createdAt', render: (_, row) => formatDateTime(row.createdAt) },
    { title: '操作', valueType: 'option', width: 100, render: (_, row) => <Button type="link" loading={execute.isPending} onClick={() => execute.mutate(row)}>执行授权</Button> },
  ]
  return <PageContainer title="临时授权">
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <ProTable<TemporaryRoleGrant> className="velora-admin-primary-table" headerTitle="授权记录" rowKey="id" columns={columns} {...grantTable} loading={grants.isLoading} search={{ labelWidth: 'auto' }} pagination={{ pageSize: 20 }} toolBarRender={canManage ? () => [<Button key="request" type="primary" icon={<PlusOutlined />} onClick={() => setRequestOpen(true)}>申请临时授权</Button>] : false} />
      {canManage && approved.length > 0 && <ProTable<ApprovalRequest> headerTitle="待执行" rowKey="id" columns={approvedColumns} dataSource={approved} search={false} pagination={false} options={false} />}
    </Space>
    <ModalForm<RequestForm> title="申请临时授权" open={requestOpen} onOpenChange={setRequestOpen} width={600} initialValues={{ validUntil: dayjs().add(8, 'hour') }} submitter={{ searchConfig: { submitText: '提交申请', resetText: '取消' } }} onFinish={async (values) => { await request.mutateAsync(values); return true }}>
      <ProForm.Item name="userId" label="用户" rules={[{ required: true, message: '请选择用户' }]}><AdminUserSelect onUserSelect={(user) => setTargetUserName(user.displayName || user.loginName)} /></ProForm.Item>
      <ProFormSelect name="roleKey" label="平台角色" options={(roles.data ?? []).map((item) => ({ label: item.name, value: item.key }))} rules={[{ required: true, message: '请选择平台角色' }]} />
      <ProFormDateTimePicker name="validUntil" label="失效时间" rules={[{ required: true, message: '请选择失效时间' }]} fieldProps={{ disabledDate: (date) => date.isBefore(dayjs(), 'day') }} />
      <ProForm.Item name="approverId" label="审批人" rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
      <ProFormTextArea name="reason" label="申请原因" fieldProps={{ maxLength: 500, showCount: true }} rules={[{ required: true, message: '请输入申请原因' }]} />
    </ModalForm>
    <ModalForm<RevokeForm> key={revoking?.id ?? 'revoke'} title="撤销临时授权" open={Boolean(revoking)} onOpenChange={(value) => !value && setRevoking(undefined)} submitter={{ searchConfig: { submitText: '确认撤销', resetText: '取消' }, submitButtonProps: { danger: true } }} onFinish={async (values) => { await revoke.mutateAsync(values); return true }}>
      <ProFormTextArea name="reason" label="撤销原因" rules={[{ required: true, message: '请输入撤销原因' }]} />
    </ModalForm>
  </PageContainer>
}
