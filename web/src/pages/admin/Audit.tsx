import { useState } from 'react'
import { Button, Drawer, Tag } from 'antd'
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { adminListAuditLogs, queryKeys } from '../../api/api'
import type { AuditLog } from '../../types'
import { auditActionLabel } from '../../labels'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'

function formatDetail(value: string): string {
  if (!value) return '无'
  try { return JSON.stringify(JSON.parse(value), null, 2) } catch { return value }
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
      {selected && <pre style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word', margin: 0 }}>{formatDetail(selected.detail)}</pre>}
    </Drawer>
  </PageContainer>
}
