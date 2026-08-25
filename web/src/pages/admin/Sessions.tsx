import { useState } from 'react'
import { App, Button, Popconfirm, Tag, Typography } from 'antd'
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { listSessions, revokeSession } from '../../api/admin-platform'
import type { AdminSession } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'
import { SYSTEM_SESSION_REVOKE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import QueryErrorState from '../../components/QueryErrorState'
import { AdminListScope, AdminListSearch } from '../../components/admin/AdminListToolbar'

export default function Sessions() {
  usePageTitle('登录会话')
  const { message } = App.useApp()
  const canRevoke = useAdminPermission(SYSTEM_SESSION_REVOKE)
  const queryClient = useQueryClient()
  const [scope, setScope] = useState('ALL')
  const [searchValue, setSearchValue] = useState('')
  const [keyword, setKeyword] = useState('')
  const sessions = useQuery({ queryKey: ['admin', 'sessions'], queryFn: listSessions, refetchInterval: 30_000 })
  const allSessions = sessions.data ?? []
  const visibleSessions = allSessions.filter((item) => (scope === 'ALL' || (scope === 'CURRENT' ? item.current : !item.current)) && (!keyword || `${item.loginName} ${item.clientIp} ${item.userAgent}`.toLowerCase().includes(keyword.toLowerCase())))
  const revoke = useMutation({ mutationFn: revokeSession, onSuccess: async () => { message.success('该设备已退出登录'); await queryClient.invalidateQueries({ queryKey: ['admin', 'sessions'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '退出登录失败') })
  const columns: ProColumns<AdminSession>[] = [
    { title: '账号', dataIndex: 'loginName', render: (_, row) => <Typography.Text strong>{row.loginName || '未知账号'}</Typography.Text> },
    { title: '登录 IP', dataIndex: 'clientIp' },
    { title: '设备', dataIndex: 'userAgent', search: false, ellipsis: true },
    { title: '最近活动', dataIndex: 'lastSeenAt', valueType: 'dateTime', search: false, render: (_, row) => formatDateTime(row.lastSeenAt) },
    { title: '自动退出时间', dataIndex: 'expiresAt', valueType: 'dateTime', search: false, render: (_, row) => formatDateTime(row.expiresAt) },
    { title: '状态', dataIndex: 'current', valueType: 'select', valueEnum: { true: { text: '当前会话' }, false: { text: '其他会话' } }, render: (_, row) => row.current ? <Tag color="processing">当前会话</Tag> : <Tag color="success">在线</Tag> },
    { title: '操作', valueType: 'option', width: 110, render: (_, row) => row.current ? <Typography.Text type="secondary">当前设备</Typography.Text> : canRevoke ? <Popconfirm title="让该设备退出登录？" description="退出后，该设备需要重新登录。" okText="退出登录" okButtonProps={{ danger: true }} onConfirm={() => revoke.mutate(row.id)}><Button type="link" danger>退出登录</Button></Popconfirm> : <Typography.Text type="secondary">—</Typography.Text> },
  ]
  return <PageContainer title="登录会话">{sessions.isError ? <QueryErrorState refetch={sessions.refetch} /> : <ProTable<AdminSession> className="velora-admin-primary-table" rowKey="id" columns={columns} dataSource={visibleSessions} loading={sessions.isLoading} search={false} pagination={{ pageSize: 20 }} polling={30_000} headerTitle={<AdminListScope value={scope} onChange={setScope} options={[{ label: '全部会话', value: 'ALL', count: allSessions.length }, { label: '当前设备', value: 'CURRENT', count: allSessions.filter((item) => item.current).length }, { label: '其他设备', value: 'OTHER', count: allSessions.filter((item) => !item.current).length }]} />} toolBarRender={() => [<AdminListSearch key="tools" value={searchValue} placeholder="搜索账号、IP 或设备" onChange={setSearchValue} onSearch={setKeyword} />]} />}</PageContainer>
}
