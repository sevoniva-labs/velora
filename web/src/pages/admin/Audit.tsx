import { useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import QueryErrorState from '../../components/QueryErrorState'
import AdminPageHead from '../../components/AdminPageHead'
import { Select, Space, Table, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { adminListAuditLogs, queryKeys } from '../../api/api'
import type { AuditLog } from '../../types'
import { auditActionLabel } from '../../labels'
import { formatDateTime } from '../../utils/format'

const ACTION_OPTIONS = [
  'LOGIN',
  'LOGOUT',
  'APPLICATION_CREATE',
  'APPLICATION_UPDATE',
  'APPLICATION_DELETE',
  'APPLICATION_LAUNCH',
  'FAVORITE_ADD',
  'FAVORITE_REMOVE',
  'PERMISSION_CHANGE',
  'CATEGORY_CREATE',
  'CATEGORY_UPDATE',
  'CATEGORY_DELETE',
  'TAG_CREATE',
  'TAG_UPDATE',
  'TAG_DELETE',
  'SETTING_UPDATE',
]

export default function AdminAudit() {
  usePageTitle('审计日志')

  const [page, setPage] = useState(1)
  const [action, setAction] = useState<string>()
  const [operator, setOperator] = useState<string>()

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: queryKeys.auditLogs({ page, action, operator }),
    queryFn: () => adminListAuditLogs({ page, pageSize: 20, action, operator }),
  })

  return (
    <div>
      <AdminPageHead
        title="审计日志"
        desc="记录登录、应用操作、收藏、启动等关键行为，满足合规追溯需求。"
      />

      <Space style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="按动作筛选"
          style={{ width: 180 }}
          value={action}
          onChange={(v) => {
            setAction(v)
            setPage(1)
          }}
          options={ACTION_OPTIONS.map((a) => ({ value: a, label: auditActionLabel(a) }))}
        />
        <Select
          allowClear
          showSearch
          placeholder="按操作人筛选"
          style={{ width: 220 }}
          value={operator}
          onChange={(v) => {
            setOperator(v)
            setPage(1)
          }}
          options={(data?.items ?? [])
            .map((l) => l.operator)
            .filter((v, i, arr) => v && arr.indexOf(v) === i)
            .map((v) => ({ value: v, label: v }))}
        />
      </Space>

      {isError ? (
        <QueryErrorState refetch={refetch} />
      ) : (
        <Table<AuditLog>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.items ?? []}
        pagination={{
          current: page,
          total: data?.total ?? 0,
          pageSize: 20,
          showSizeChanger: false,
          onChange: setPage,
        }}
        size="middle"
        columns={[
          { title: '时间', dataIndex: 'createdAt', width: 170, render: (v: string) => formatDateTime(v) },
          { title: '操作人', dataIndex: 'operator', width: 160, render: (v: string) => v || '-' },
          {
            title: '动作',
            dataIndex: 'action',
            width: 140,
            render: (v: string) => <Tag color="blue">{auditActionLabel(v)}</Tag>,
          },
          { title: '资源', dataIndex: 'resource', width: 120, render: (v: string) => v || '-' },
          { title: '资源 ID', dataIndex: 'resourceId', width: 100, render: (v: string) => v || '-' },
          { title: 'IP', dataIndex: 'ip', width: 130 },
          { title: 'Request ID', dataIndex: 'requestId', width: 130, render: (v: string) => (v ? <Typography.Text code>{v.slice(0, 8)}</Typography.Text> : '-') },
        ]}
      />
      )}
    </div>
  )
}
