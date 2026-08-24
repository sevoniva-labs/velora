import { useState } from 'react'
import { App, Button, Popconfirm, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { DrawerForm, PageContainer, ProForm, ProFormSelect, ProFormText, ProFormTextArea, ProList, type ProColumns } from '@ant-design/pro-components'
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
  const updateStatus = useMutation({
    mutationFn: (group: UserGroup) => updateUserGroup(group.id, { groupKey: group.groupKey, name: group.name, description: group.description, status: group.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE' }),
    onSuccess: async (group) => { message.success(group.status === 'ACTIVE' ? '用户组已启用' : '用户组已停用'); await queryClient.invalidateQueries({ queryKey: ['admin', 'user-groups'] }) },
    onError: (error) => message.error(error instanceof Error ? error.message : '用户组状态更新失败'),
  })
  const columns: ProColumns<UserGroup>[] = [
    { title: '用户组', dataIndex: 'name', listSlot: 'title', render: (_, row) => <Typography.Text strong>{row.name}</Typography.Text> },
    { dataIndex: 'groupKey', listSlot: 'description', search: false, render: (_, row) => <Typography.Text type="secondary">{row.groupKey}{row.description ? ` · ${row.description}` : ''}</Typography.Text> },
    { title: '状态', dataIndex: 'status', listSlot: 'subTitle', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } }, render: (_, row) => <Tag color={row.status === 'ACTIVE' ? 'success' : 'default'}>{row.status === 'ACTIVE' ? '启用' : '停用'}</Tag> },
    { dataIndex: 'memberCount', listSlot: 'content', search: false, render: (_, row) => <Space wrap><Typography.Text>{row.memberCount} 名成员</Typography.Text>{row.roles.length ? row.roles.map((role) => <Tag key={role}>{roles.data?.find((item) => item.key === role)?.name ?? role}</Tag>) : <Typography.Text type="secondary">未分配平台角色</Typography.Text>}</Space> },
  ]
  if (canManage) columns.push({ title: '操作', listSlot: 'actions', valueType: 'option', search: false, render: (_, row) => [<Button key="edit" type="link" onClick={() => { setEditing(row); setOpen(true) }}>编辑</Button>, <Popconfirm key="status" title={row.status === 'ACTIVE' ? '停用此用户组？' : '启用此用户组？'} description={row.status === 'ACTIVE' ? '停用后，该组不再授予平台角色。成员关系会保留。' : undefined} okText={row.status === 'ACTIVE' ? '停用' : '启用'} okButtonProps={{ danger: row.status === 'ACTIVE' }} onConfirm={() => updateStatus.mutate(row)}><Button type="link" danger={row.status === 'ACTIVE'}>{row.status === 'ACTIVE' ? '停用' : '启用'}</Button></Popconfirm>] })

  return <PageContainer title="用户组">
    {groups.isError ? <QueryErrorState refetch={groups.refetch} /> : <ProList<UserGroup> className="velora-admin-primary-table velora-admin-entity-list" rowKey="id" columns={columns} {...groupTable} loading={groups.isLoading} search={{ filterType: 'light' }} pagination={{ pageSize: 20 }} toolBarRender={canManage ? () => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(undefined); setOpen(true) }}>新建用户组</Button>] : false} />}
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
