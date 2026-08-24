import { Tag } from 'antd'
import { ProTable, type ProColumns } from '@ant-design/pro-components'
import { adminListAuditLogs } from '../../api/api'
import type { AuditLog } from '../../types'
import { auditActionLabel } from '../../labels'
import { formatDateTime } from '../../utils/format'

interface Props { applicationId: string }

export default function ApplicationHistory({ applicationId }: Props) {
  const columns: ProColumns<AuditLog>[] = [
    { title: '时间', dataIndex: 'createdAt', render: (_, row) => formatDateTime(row.createdAt), width: 180 },
    { title: '操作', dataIndex: 'action', render: (_, row) => <Tag>{auditActionLabel(row.action)}</Tag> },
    { title: '操作人', dataIndex: 'operator' },
    { title: '来源 IP', dataIndex: 'ip', render: (_, row) => row.ip || '—' },
  ]
  return <ProTable<AuditLog> className="velora-admin-secondary-table" rowKey="id" columns={columns} request={async (params) => { const data = await adminListAuditLogs({ page: params.current, pageSize: params.pageSize, resourceId: applicationId }); return { data: data.items, total: data.total, success: true } }} search={false} options={false} pagination={{ defaultPageSize: 20, showSizeChanger: true }} />
}
