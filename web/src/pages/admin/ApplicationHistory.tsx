import { Tag } from 'antd'
import { ProTable, type ProColumns } from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { adminListAuditLogs } from '../../api/api'
import type { AuditLog } from '../../types'
import { auditActionLabel } from '../../labels'
import { formatDateTime } from '../../utils/format'

interface Props { applicationId: string }

export default function ApplicationHistory({ applicationId }: Props) {
  const logs = useQuery({ queryKey: ['admin', 'applications', applicationId, 'history'], queryFn: () => adminListAuditLogs({ page: 1, pageSize: 500 }) })
  const items = (logs.data?.items ?? []).filter((item) => String(item.resourceId) === applicationId)
  const columns: ProColumns<AuditLog>[] = [
    { title: '时间', dataIndex: 'createdAt', render: (_, row) => formatDateTime(row.createdAt), width: 180 },
    { title: '操作', dataIndex: 'action', render: (_, row) => <Tag>{auditActionLabel(row.action)}</Tag> },
    { title: '操作人', dataIndex: 'operator' },
    { title: '请求编号', dataIndex: 'requestId', copyable: true, ellipsis: true },
  ]
  return <ProTable<AuditLog> rowKey="id" columns={columns} dataSource={items} loading={logs.isLoading} search={{ labelWidth: 'auto' }} pagination={{ pageSize: 20 }} />
}
