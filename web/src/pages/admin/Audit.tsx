import { useState } from 'react'
import type { ReactNode } from 'react'
import { Button, Drawer, Empty, Tag, Typography } from 'antd'
import { PageContainer, ProDescriptions, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { adminListAuditLogs, queryKeys } from '../../api/api'
import type { AuditLog } from '../../types'
import { auditActionLabel } from '../../labels'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'

function parseDetail(value: string): Record<string, unknown> {
  if (!value) return {}
  try { const parsed = JSON.parse(value); return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {} } catch { return {} }
}

export default function AdminAudit() {
  usePageTitle('操作审计')
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<{ action?: string; operator?: string }>({})
  const [selected, setSelected] = useState<AuditLog>()
  const logs = useQuery({ queryKey: queryKeys.auditLogs({ page, ...filters }), queryFn: () => adminListAuditLogs({ page, pageSize: 20, ...filters }) })
  const columns: ProColumns<AuditLog>[] = [
    { title: '时间', dataIndex: 'createdAt', valueType: 'dateTime', search: false, width: 180, render: (_, row) => formatDateTime(row.createdAt) },
    { title: '操作人', dataIndex: 'operator', width: 160 },
    { title: '操作', dataIndex: 'action', valueType: 'select', valueEnum: Object.fromEntries(Array.from(new Set((logs.data?.items ?? []).map((item) => item.action))).map((action) => [action, { text: auditActionLabel(action) }])), render: (_, row) => <Tag color="blue">{auditActionLabel(row.action)}</Tag> },
    { title: '对象', dataIndex: 'resource', search: false, width: 140, render: (_, row) => row.resource || '—' },
    { title: '来源 IP', dataIndex: 'ip', search: false, width: 140 },
    { title: '详情', valueType: 'option', width: 80, render: (_, row) => <Button type="link" onClick={() => setSelected(row)}>查看</Button> },
  ]
  return <PageContainer title="操作审计">
    <ProTable<AuditLog> rowKey="id" columns={columns} dataSource={logs.data?.items ?? []} loading={logs.isLoading} search={{ labelWidth: 'auto' }} options={false} onSubmit={(values) => { setFilters({ action: values.action as string | undefined, operator: values.operator as string | undefined }); setPage(1) }} onReset={() => { setFilters({}); setPage(1) }} pagination={{ current: page, pageSize: 20, total: logs.data?.total ?? 0, showSizeChanger: false, onChange: setPage }} />
    <Drawer title="审计详情" open={Boolean(selected)} onClose={() => setSelected(undefined)} width={640}>
      {selected && <AuditDetail log={selected} />}
    </Drawer>
  </PageContainer>
}

const DETAIL_LABELS: Record<string, string> = {
  name: '名称', status: '状态', roles: '角色', permissions: '权限', data_scope: '数据范围', department_count: '部门数', member_count: '成员数', assignment_count: '任职数', role_count: '应用角色数', effective_users: '生效用户', added_users: '新增用户', revoked_users: '撤销用户', role_changed_users: '角色变化用户', privileged_users: '高权限用户', retried_messages: '重试任务', reason: '原因', decision: '处理结果', version: '版本', sensitive: '敏感配置', rotate_secret: '轮换密钥', force_change: '下次登录修改密码', valid_until: '有效期至', before: '变更前', after: '变更后',
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
