import { useState } from 'react'
import { Tag } from 'antd'
import { ProTable, type ProColumns } from '@ant-design/pro-components'
import { adminListAuditLogs } from '../../api/api'
import type { AuditLog } from '../../types'
import { auditActionLabel } from '../../labels'
import { formatDateTime } from '../../utils/format'
import QueryErrorState from '../../components/QueryErrorState'

interface Props { applicationId: string }

export default function ApplicationHistory({ applicationId }: Props) {
  const [loadError, setLoadError] = useState(false)
  const columns: ProColumns<AuditLog>[] = [
    { title: '时间', dataIndex: 'createdAt', render: (_, row) => formatDateTime(row.createdAt), width: 180 },
    { title: '操作', dataIndex: 'action', render: (_, row) => <Tag>{auditActionLabel(row.action)}</Tag> },
    { title: '操作人', dataIndex: 'operator' },
    { title: '来源 IP', dataIndex: 'ip', render: (_, row) => row.ip || '—' },
  ]
  return loadError ? <QueryErrorState compact refetch={() => setLoadError(false)} /> : <ProTable<AuditLog> className="velora-admin-secondary-table" rowKey="id" columns={columns} request={async (params) => { try { const data = await adminListAuditLogs({ page: params.current, pageSize: params.pageSize, resourceId: applicationId }); return { data: data.items, total: data.total, success: true } } catch { setLoadError(true); return { data: [], total: 0, success: false } } }} search={false} options={false} pagination={{ defaultPageSize: 20, showSizeChanger: true }} />
}
