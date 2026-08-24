import { useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { DrawerForm, PageContainer, ProForm, ProFormSelect, ProFormText, ProFormTextArea, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createUserGroup, listPlatformRoles, listUserGroups, replaceUserGroupMembers, replaceUserGroupRoles, updateUserGroup, type UserGroupInput } from '../../api/admin-platform'
import type { UserGroup } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import AdminUserSelect from '../../components/AdminUserSelect'
import { SYSTEM_USER_GROUP_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import { useClientTableSearch } from '../../utils/tableSearch'
import QueryErrorState from '../../components/QueryErrorState'

interface GroupForm extends UserGroupInput { memberIds: string[]; roles: string[] }

export default function UserGroups() {
  usePageTitle('用户组')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const canManage = useAdminPermission(SYSTEM_USER_GROUP_MANAGE)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<UserGroup>()
  const groups = useQuery({ queryKey: ['admin', 'user-groups'], queryFn: listUserGroups })
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const groupTable = useClientTableSearch(groups.data ?? [], { exact: ['status'] })
  const mutation = useMutation({
    mutationFn: async (values: GroupForm) => {
      const payload = { name: values.name, description: values.description ?? '', status: values.status, groupKey: values.groupKey }
      const group = editing ? await updateUserGroup(editing.id, payload) : await createUserGroup(payload)
      await Promise.all([replaceUserGroupMembers(group.id, values.memberIds ?? []), replaceUserGroupRoles(group.id, values.roles ?? [])])
      return group
    },
    onSuccess: async () => { message.success(editing ? '用户组已更新' : '用户组已创建'); setOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'user-groups'] }) },
    onError: (error) => message.error(error instanceof Error ? error.message : '用户组保存失败'),
  })
  const columns: ProColumns<UserGroup>[] = [
    { title: '用户组', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.groupKey}</Typography.Text></Space> },
    { title: '成员', dataIndex: 'memberCount', search: false, width: 100, render: (_, row) => `${row.memberCount} 人` },
    { title: '平台角色', dataIndex: 'roles', search: false, render: (_, row) => row.roles.length ? row.roles.map((role) => <Tag key={role}>{roles.data?.find((item) => item.key === role)?.name ?? role}</Tag>) : '—' },
    { title: '说明', dataIndex: 'description', search: false, ellipsis: true },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } }, render: (_, row) => row.status === 'ACTIVE' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag> },
  ]
  if (canManage) columns.push({ title: '操作', valueType: 'option', width: 90, render: (_, row) => <Button type="link" onClick={() => { setEditing(row); setOpen(true) }}>编辑</Button> })

  return <PageContainer title="用户组">
    {groups.isError ? <QueryErrorState refetch={groups.refetch} /> : <ProTable<UserGroup> className="velora-admin-primary-table" rowKey="id" columns={columns} {...groupTable} loading={groups.isLoading} search={{ labelWidth: 'auto' }} pagination={{ pageSize: 20 }} toolBarRender={canManage ? () => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(undefined); setOpen(true) }}>新建用户组</Button>] : false} />}
    <DrawerForm<GroupForm> key={editing?.id ?? 'new'} title={editing ? '编辑用户组' : '新建用户组'} open={open} onOpenChange={setOpen} width={560} initialValues={editing ?? { status: 'ACTIVE', memberIds: [], roles: [] }} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await mutation.mutateAsync(values); return true }}>
      {!editing && <ProFormText name="groupKey" label="用户组编码" rules={[{ required: true, message: '请输入用户组编码' }, { pattern: /^[a-z][a-z0-9_-]{1,63}$/, message: '使用小写字母、数字、短横线或下划线' }]} />}
      <ProFormText name="name" label="用户组名称" rules={[{ required: true, message: '请输入用户组名称' }]} />
      <ProFormTextArea name="description" label="说明" fieldProps={{ maxLength: 200, showCount: true }} />
      <ProForm.Item name="memberIds" label="成员"><AdminUserSelect mode="multiple" /></ProForm.Item>
      <ProFormSelect name="roles" label="平台角色" mode="multiple" options={(roles.data ?? []).map((role) => ({ label: role.name, value: role.key }))} />
      <ProFormSelect name="status" label="状态" options={[{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'DISABLED' }]} rules={[{ required: true }]} />
    </DrawerForm>
  </PageContainer>
}
