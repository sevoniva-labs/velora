import { useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { App, Button, Drawer, Empty, Space, Tag, Typography } from 'antd'
import { ModalForm, PageContainer, ProDescriptions, ProForm, ProFormDigit, ProFormSelect, ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminListAuditLogs } from '../../api/api'
import type { AuditLog } from '../../types'
import { AUDIT_ACTION_LABEL, AUDIT_RESOURCE_LABEL, auditActionLabel, auditResourceLabel } from '../../labels'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'
import { createApproval, exportAuditLogs, listApprovals, verifyAuditIntegrity } from '../../api/admin-platform'
import AdminUserSelect from '../../components/AdminUserSelect'
import { useMe } from '../../auth/useMe'
import QueryErrorState from '../../components/QueryErrorState'

interface ExportForm { format: 'json' | 'csv'; limit: number; approverId: string }

function parseDetail(value: string): Record<string, unknown> {
  if (!value) return {}
  try { const parsed = JSON.parse(value); return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {} } catch { return {} }
}

function isoDate(value: unknown): string | undefined {
  if (!value) return undefined
  if (typeof value === 'object' && 'toISOString' in value && typeof value.toISOString === 'function') return value.toISOString()
  const date = new Date(String(value))
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

export default function AdminAudit() {
  usePageTitle('操作记录')
  const { message } = App.useApp()
  const me = useMe()
  const queryClient = useQueryClient()
  const actionRef = useRef<ActionType>(null)
  const [loadError, setLoadError] = useState(false)
  const [selected, setSelected] = useState<AuditLog>()
  const [exportOpen, setExportOpen] = useState(false)
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals })
  const approvedExport = (approvals.data ?? []).find((item) => item.action === 'audit.export' && item.resourceId === 'organization' && item.status === 'APPROVED')
  const integrity = useMutation({ mutationFn: verifyAuditIntegrity, onSuccess: (verified) => verified ? message.success('操作记录完整性校验通过') : message.error('操作记录完整性校验失败'), onError: (error) => message.error(error instanceof Error ? error.message : '完整性校验失败') })
  const requestExport = useMutation({ mutationFn: (values: ExportForm) => createApproval({ requestType: 'AUDIT_LOG_EXPORT', action: 'audit.export', resource: 'audit_log', resourceId: 'organization', summary: `导出最近 ${values.limit} 条操作记录`, payloadJson: JSON.stringify({ format: values.format, limit: values.limit }), approverIds: [values.approverId] }), onSuccess: async () => { message.success('导出申请已提交'); setExportOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '导出申请提交失败') })
  const executeExport = useMutation({ mutationFn: async () => { const payload = JSON.parse(approvedExport!.payloadJson) as { format?: 'json' | 'csv'; limit?: number }; const result = await exportAuditLogs(payload.format ?? 'csv', payload.limit ?? 1000, approvedExport!.id); const binary = atob(result.content); const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0)); const href = URL.createObjectURL(new Blob([bytes], { type: result.contentType })); const link = document.createElement('a'); link.href = href; link.download = result.filename || 'audit-logs.csv'; link.click(); URL.revokeObjectURL(href) }, onSuccess: async () => { message.success('操作记录已导出'); await queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '导出失败') })
  const columns: ProColumns<AuditLog>[] = [
    { title: '操作时间', dataIndex: 'occurredAt', valueType: 'dateTimeRange', hideInTable: true },
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', search: false, width: 180, render: (_, row) => formatDateTime(row.createdAt) },
    { title: '操作人', dataIndex: 'operator', width: 160 },
    { title: '操作', dataIndex: 'action', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(AUDIT_ACTION_LABEL).map(([value, text]) => [value, { text }])), render: (_, row) => <Tag color="blue">{auditActionLabel(row.action)}</Tag> },
    { title: '操作对象', dataIndex: 'resource', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(AUDIT_RESOURCE_LABEL).map(([value, text]) => [value, { text }])), width: 140, render: (_, row) => row.resource ? auditResourceLabel(row.resource) : '—' },
    { title: '结果', dataIndex: 'result', valueType: 'select', valueEnum: { SUCCESS: { text: '成功' }, FAILED: { text: '失败' } }, width: 90, render: (_, row) => <Tag color={row.result === 'SUCCESS' ? 'success' : 'error'}>{row.result === 'SUCCESS' ? '成功' : '失败'}</Tag> },
    { title: '来源 IP', dataIndex: 'ip', search: false, width: 140 },
    { title: '详情', valueType: 'option', width: 80, render: (_, row) => <Button type="link" onClick={() => setSelected(row)}>查看</Button> },
  ]
  return <PageContainer title="操作记录" extra={<Space><Button loading={integrity.isPending} onClick={() => integrity.mutate()}>校验完整性</Button>{approvedExport ? <Button type="primary" loading={executeExport.isPending} onClick={() => executeExport.mutate()}>导出已批准记录</Button> : <Button type="primary" onClick={() => setExportOpen(true)}>导出</Button>}</Space>}>
    {loadError ? <QueryErrorState refetch={() => { setLoadError(false); actionRef.current?.reload() }} /> : <ProTable<AuditLog> className="velora-admin-primary-table" actionRef={actionRef} rowKey="id" columns={columns} search={{ filterType: 'light' }} options={false} request={async (params) => { try { const occurredAt = Array.isArray(params.occurredAt) ? params.occurredAt : []; const data = await adminListAuditLogs({ page: params.current, pageSize: params.pageSize, operator: String(params.operator ?? '').trim() || undefined, action: String(params.action ?? '') || undefined, resourceType: String(params.resource ?? '') || undefined, result: String(params.result ?? '') || undefined, from: isoDate(occurredAt[0]), to: isoDate(occurredAt[1]) }); return { data: data.items, total: data.total, success: true } } catch { setLoadError(true); return { data: [], total: 0, success: false } } }} pagination={{ defaultPageSize: 20, showSizeChanger: false }} />}
    <Drawer title="操作详情" open={Boolean(selected)} onClose={() => setSelected(undefined)} width={640}>
      {selected && <AuditDetail log={selected} />}
    </Drawer>
    <ModalForm<ExportForm> title="申请导出操作记录" open={exportOpen} onOpenChange={setExportOpen} initialValues={{ format: 'csv', limit: 1000 }} submitter={{ searchConfig: { submitText: '提交审批', resetText: '取消' } }} onFinish={async (values) => { await requestExport.mutateAsync(values); return true }}>
      <ProFormSelect name="format" label="文件格式" options={[{ value: 'csv', label: 'CSV' }, { value: 'json', label: 'JSON' }]} rules={[{ required: true }]} />
      <ProFormDigit name="limit" label="记录数量" min={1} max={5000} fieldProps={{ precision: 0 }} rules={[{ required: true, message: '请输入记录数量' }]} />
      <ProForm.Item name="approverId" label="审批人" rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
    </ModalForm>
  </PageContainer>
}

const DETAIL_LABELS: Record<string, string> = {
  name: '名称', status: '状态', roles: '角色', permissions: '权限', data_scope: '可管理范围', department_count: '部门数', member_count: '成员数', assignment_count: '任职数', role_count: '应用角色数', effective_users: '可使用用户', added_users: '新增用户', revoked_users: '移除用户', role_changed_users: '角色变化用户', privileged_users: '高权限用户', retried_messages: '重试任务', reason: '原因', decision: '处理结果', version: '版本', sensitive: '包含密码或密钥', rotate_secret: '更换密钥', force_change: '下次登录修改密码', valid_until: '有效期至', before: '变更前', after: '变更后',
}

function AuditDetail({ log }: { log: AuditLog }) {
  const detail = parseDetail(log.detail)
  const visible = Object.entries(detail).filter(([key]) => !key.endsWith('_id') && key !== 'provider')
  return <>
    <ProDescriptions column={1} dataSource={log} columns={[{ title: '时间', render: () => formatDateTime(log.createdAt) }, { title: '操作人', render: () => log.operator || '系统' }, { title: '操作', render: () => auditActionLabel(log.action) }, { title: '对象', render: () => log.resource || '—' }, { title: '来源 IP', render: () => log.ip || '—' }]} />
    <Typography.Title level={5}>变更内容</Typography.Title>
    {visible.length ? <ProDescriptions column={1} dataSource={detail} columns={visible.map(([key, value]) => ({ title: DETAIL_LABELS[key] ?? readableKey(key), render: () => detailValue(value) }))} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="本次操作没有字段变更" />}
  </>
}

function detailValue(value: unknown): ReactNode {
  if (typeof value === 'boolean') return value ? '是' : '否'
  if (Array.isArray(value)) return value.length ? value.map(String).join('、') : '无'
  if (value && typeof value === 'object') return Object.entries(value as Record<string, unknown>).map(([key, child]) => `${DETAIL_LABELS[key] ?? readableKey(key)}：${String(child)}`).join('；')
  return value == null || value === '' ? '—' : String(value)
}

function readableKey(key: string): string { return key.split('_').filter(Boolean).join(' ') }
