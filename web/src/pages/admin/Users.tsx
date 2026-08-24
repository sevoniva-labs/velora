import { useMemo, useRef, useState } from 'react'
import { App, Button, Popconfirm, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProFormSelect, ProFormText, ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { adminCreateUser, adminPageUsers, adminUpdateUserStatus, type CreateAdminUserInput } from '../../api/api'
import { listPlatformRoles } from '../../api/admin-platform'
import type { AdminUser } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { SYSTEM_USER_CREATE, SYSTEM_USER_UPDATE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'

type CreateForm = Omit<CreateAdminUserInput, 'entitlements'>

function statusTag(status: AdminUser['status']) {
  if (status === 'ACTIVE') return <Tag color="success">正常</Tag>
  if (status === 'LOCKED') return <Tag color="warning">已锁定</Tag>
  return <Tag>已停用</Tag>
}

export default function Users() {
  usePageTitle('用户')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const actionRef = useRef<ActionType>(null)
  const canCreate = useAdminPermission(SYSTEM_USER_CREATE)
  const canUpdate = useAdminPermission(SYSTEM_USER_UPDATE)
  const [createOpen, setCreateOpen] = useState(false)
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const roleNames = useMemo(() => new Map((roles.data ?? []).map((role) => [role.key, role.name])), [roles.data])
  const create = useMutation({
    mutationFn: (values: CreateForm) => adminCreateUser({ ...values, loginName: values.loginName.trim(), displayName: values.displayName.trim(), email: values.email?.trim(), entitlements: [] }),
    onSuccess: async () => { message.success('用户已创建'); setCreateOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }); actionRef.current?.reload() },
    onError: (error) => message.error(error instanceof Error ? error.message : '用户创建失败'),
  })
  const updateStatus = useMutation({
    mutationFn: ({ user, status }: { user: AdminUser; status: 'ACTIVE' | 'DISABLED' }) => adminUpdateUserStatus(user.id, status),
    onSuccess: async (_, values) => { message.success(values.status === 'ACTIVE' ? '用户已启用' : '用户已停用'); await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }); actionRef.current?.reload() },
    onError: (error) => message.error(error instanceof Error ? error.message : '用户状态更新失败'),
  })
  const columns: ProColumns<AdminUser>[] = [
    { title: '用户', dataIndex: 'keyword', render: (_, row) => <Space direction="vertical" size={0}><Link to={`/admin/users/${row.id}`}><Typography.Text strong>{row.displayName || row.loginName}</Typography.Text></Link><Typography.Text type="secondary">{row.loginName}</Typography.Text></Space> },
    { title: '邮箱', dataIndex: 'email', search: false, render: (_, row) => row.email || '—' },
    { title: '平台角色', dataIndex: 'roleKey', valueType: 'select', valueEnum: Object.fromEntries((roles.data ?? []).map((role) => [role.key, role.name])), render: (_, row) => row.roles.length ? row.roles.map((role) => <Tag key={role}>{roleNames.get(role) ?? role}</Tag>) : '—' },
    { title: '有效应用', dataIndex: 'entitlements', search: false, width: 110, render: (_, row) => `${row.entitlements.filter((item) => item.status === 'ACTIVE').length} 个` },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '正常' }, DISABLED: { text: '已停用' }, LOCKED: { text: '已锁定' } }, render: (_, row) => statusTag(row.status) },
    { title: '操作', valueType: 'option', width: canUpdate ? 150 : 70, render: (_, row) => <Space><Link to={`/admin/users/${row.id}`}>查看</Link>{canUpdate && <Popconfirm title={row.status === 'ACTIVE' ? '停用此用户？' : '启用此用户？'} description={row.status === 'ACTIVE' ? '停用后将撤销登录会话和应用访问。' : undefined} okText={row.status === 'ACTIVE' ? '停用' : '启用'} okButtonProps={{ danger: row.status === 'ACTIVE' }} onConfirm={() => updateStatus.mutate({ user: row, status: row.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE' })}><Button type="link" danger={row.status === 'ACTIVE'}>{row.status === 'ACTIVE' ? '停用' : '启用'}</Button></Popconfirm>}</Space> },
  ]

  return <PageContainer title="用户">
    <ProTable<AdminUser> actionRef={actionRef} rowKey="id" columns={columns} request={async (params) => { const data = await adminPageUsers({ page: params.current, pageSize: params.pageSize, keyword: String(params.keyword ?? '') || undefined, status: String(params.status ?? '') || undefined, roleKey: String(params.roleKey ?? '') || undefined }); return { data: data.items, total: data.total, success: true } }} search={{ labelWidth: 'auto' }} pagination={{ defaultPageSize: 20, showSizeChanger: true }} toolBarRender={canCreate ? () => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>新建用户</Button>] : false} />
    <ModalForm<CreateForm> title="新建用户" open={createOpen} onOpenChange={setCreateOpen} width={620} initialValues={{ roles: ['user'] }} submitter={{ searchConfig: { submitText: '创建用户', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
      <ProFormText name="loginName" label="登录账号" rules={[{ required: true, message: '请输入登录账号' }, { pattern: /^[a-zA-Z][a-zA-Z0-9._-]{2,63}$/, message: '3–64 位，以字母开头' }]} />
      <ProFormText name="displayName" label="姓名" rules={[{ required: true, message: '请输入姓名' }]} />
      <ProFormText name="email" label="邮箱" rules={[{ type: 'email', message: '请输入有效邮箱' }]} />
      <ProFormText.Password name="password" label="初始密码" rules={[{ required: true, message: '请输入初始密码' }, { min: 12, message: '密码至少 12 位' }]} />
      <ProFormSelect name="roles" label="平台角色" mode="multiple" options={(roles.data ?? []).map((role) => ({ label: role.name, value: role.key }))} rules={[{ required: true, message: '请选择平台角色' }]} />
    </ModalForm>
  </PageContainer>
}
