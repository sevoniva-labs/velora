import { useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { ModalForm, PageContainer, ProFormRadio, ProFormTextArea, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { decideApproval, listApprovals } from '../../api/admin-platform'
import type { ApprovalRequest } from '../../types'
import { useMe } from '../../auth/useMe'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'
import { approvalTypeLabel } from '../../labels'
import { APPROVAL_TASK_DECIDE, hasPermission } from '../../auth/permissions'
import { useClientTableSearch } from '../../utils/tableSearch'
import QueryErrorState from '../../components/QueryErrorState'

interface DecisionForm { decision: 'APPROVE' | 'REJECT'; comment: string }
const STATUS_LABELS: Record<string, string> = { PENDING: '待审批', APPROVED: '已批准', REJECTED: '已拒绝', WITHDRAWN: '已撤回', EXPIRED: '已过期', EXECUTED: '已执行' }

export default function Approvals() {
  usePageTitle('审批')
  const { message } = App.useApp()
  const me = useMe()
  const canDecide = hasPermission(me.data?.permissions, APPROVAL_TASK_DECIDE, me.data?.roles)
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [selected, setSelected] = useState<ApprovalRequest>()
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, refetchInterval: 30_000 })
  const approvalTable = useClientTableSearch(approvals.data ?? [], { exact: ['status'] })
  const decide = useMutation({ mutationFn: (values: DecisionForm) => decideApproval(selected!.id, values.decision, values.comment ?? ''), onSuccess: async (_, values) => { message.success(values.decision === 'APPROVE' ? '已批准' : '已拒绝'); setSelected(undefined); await queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '审批处理失败') })
  const columns: ProColumns<ApprovalRequest>[] = [
    { title: '事项', dataIndex: 'summary', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.summary}</Typography.Text><Typography.Text type="secondary">{approvalTypeLabel(row.requestType)}</Typography.Text></Space> },
    { title: '申请时间', dataIndex: 'createdAt', valueType: 'dateTime', render: (_, row) => formatDateTime(row.createdAt) },
    { title: '到期时间', dataIndex: 'expiresAt', valueType: 'dateTime', search: false, render: (_, row) => formatDateTime(row.expiresAt) },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(STATUS_LABELS).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag color={row.status === 'PENDING' ? 'processing' : row.status === 'APPROVED' ? 'success' : 'default'}>{STATUS_LABELS[row.status] ?? '已结束'}</Tag> },
    { title: '操作', valueType: 'option', width: 110, render: (_, row) => canDecide && row.status === 'PENDING' && row.tasks.some((task) => task.assigneeId === me.data?.id && task.status === 'PENDING') ? <Button type="link" onClick={() => setSelected(row)}>处理</Button> : row.status === 'APPROVED' && executionPath(row) ? <Button type="link" onClick={() => navigate(executionPath(row)!)}>前往执行</Button> : <Typography.Text type="secondary">—</Typography.Text> },
  ]
  return <PageContainer title="审批">{approvals.isError ? <QueryErrorState refetch={approvals.refetch} /> : <ProTable<ApprovalRequest> className="velora-admin-primary-table" rowKey="id" columns={columns} {...approvalTable} loading={approvals.isLoading} search={{ filterType: 'light' }} pagination={{ pageSize: 20 }} />}
    <ModalForm<DecisionForm> key={selected?.id ?? 'decision'} title="处理审批" open={Boolean(selected)} onOpenChange={(value) => !value && setSelected(undefined)} initialValues={{ decision: 'APPROVE' }} submitter={{ searchConfig: { submitText: '提交', resetText: '取消' } }} onFinish={async (values) => { await decide.mutateAsync(values); return true }}>
      <ProFormRadio.Group name="decision" label="处理结果" options={[{ label: '批准', value: 'APPROVE' }, { label: '拒绝', value: 'REJECT' }]} rules={[{ required: true }]} />
      <ProFormTextArea name="comment" label="处理意见" fieldProps={{ maxLength: 500, showCount: true }} />
    </ModalForm>
  </PageContainer>
}

function executionPath(request: ApprovalRequest): string | undefined {
  if (request.resource === 'user') return `/admin/users/${encodeURIComponent(request.resourceId)}`
  if (request.resource === 'portal_application') return `/admin/applications/${encodeURIComponent(request.resourceId)}`
  if (request.action === 'audit.export') return '/admin/audit'
  if (request.action.includes('security')) return '/admin/login-security'
  if (request.resource === 'role') return '/admin/roles'
  if (request.resource === 'config_change') return '/admin/config-changes'
  return undefined
}
