import { useState } from 'react'
import { App, Button, Popconfirm, Space, Tag, Typography } from 'antd'
import { ModalForm, PageContainer, ProForm, ProFormCheckbox, ProFormRadio, ProFormSelect, ProFormText, ProList, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { copyPlatformRole, createApproval, createPlatformRole, listApprovals, listDepartments, listPlatformPermissions, listPlatformRoles, updatePlatformRole, updateRoleDataScope, updateRolePermissions } from '../../api/admin-platform'
import type { ApprovalRequest, PlatformRole } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMe } from '../../auth/useMe'
import AdminUserSelect from '../../components/AdminUserSelect'
import { SYSTEM_ROLE_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import QueryErrorState from '../../components/QueryErrorState'
import { AdminListScope, AdminListSearch } from '../../components/admin/AdminListToolbar'

const DATA_SCOPE_LABELS: Record<string, string> = { ALL: '全部数据', DEPARTMENT: '指定部门', SELF_DEPARTMENT: '本部门', SELF: '仅本人' }
interface RoleForm { roleKey: string; name: string; description?: string }

export default function Roles() {
  usePageTitle('平台角色')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const me = useMe()
  const canManage = useAdminPermission(SYSTEM_ROLE_MANAGE)
  const [permissionRole, setPermissionRole] = useState<PlatformRole>()
  const [scopeRole, setScopeRole] = useState<PlatformRole>()
  const [createOpen, setCreateOpen] = useState(false)
  const [editRole, setEditRole] = useState<PlatformRole>()
  const [copyRole, setCopyRole] = useState<PlatformRole>()
  const [scope, setScope] = useState('ALL')
  const [searchValue, setSearchValue] = useState('')
  const [keyword, setKeyword] = useState('')
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const roleRows = roles.data ?? []
  const visibleRoles = roleRows.filter((role) => {
    if (scope !== 'ALL' && role.status !== scope) return false
    return !keyword || `${role.name} ${role.key} ${role.description}`.toLocaleLowerCase('zh-CN').includes(keyword.toLocaleLowerCase('zh-CN'))
  })
  const permissions = useQuery({ queryKey: ['admin', 'permissions'], queryFn: listPlatformPermissions })
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canManage })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'roles'] })
  const refreshAll = async () => { await Promise.all([refresh(), queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] })]) }
  const permissionRequest = useMutation({ mutationFn: (values: { permissions: string[]; approverId: string }) => { const permissions = normalized(values.permissions); return createApproval({ requestType: 'ROLE_PERMISSION_CHANGE', action: 'role.permissions.update', resource: 'role', resourceId: permissionRole!.key, summary: `更新平台角色“${permissionRole!.name}”的权限`, payloadJson: JSON.stringify({ permissions }), approverIds: [values.approverId] }) }, onSuccess: async () => { message.success('角色权限变更已提交审批'); setPermissionRole(undefined); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '审批提交失败') })
  const scopeRequest = useMutation({ mutationFn: (values: { dataScope: string; departmentIds?: string[]; approverId: string }) => { const departmentIds = normalized(values.departmentIds ?? []); return createApproval({ requestType: 'ROLE_DATA_SCOPE_CHANGE', action: 'role.data_scope.update', resource: 'role', resourceId: scopeRole!.key, summary: `更新平台角色“${scopeRole!.name}”的可管理范围`, payloadJson: JSON.stringify({ data_scope: values.dataScope.trim(), department_ids: departmentIds }), approverIds: [values.approverId] }) }, onSuccess: async () => { message.success('可管理范围变更已提交审批'); setScopeRole(undefined); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '审批提交失败') })
  const execute = useMutation({ mutationFn: async (approval: ApprovalRequest) => { const payload = JSON.parse(approval.payloadJson) as { permissions?: string[]; data_scope?: string; department_ids?: string[] }; if (approval.action === 'role.permissions.update') return updateRolePermissions(approval.resourceId, payload.permissions ?? [], approval.id); return updateRoleDataScope(approval.resourceId, payload.data_scope ?? 'SELF', payload.department_ids ?? [], approval.id) }, onSuccess: async () => { message.success('角色变更已生效'); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色变更执行失败') })
  const approved = (approvals.data ?? []).filter((item) => item.resource === 'role' && item.status === 'APPROVED' && (item.action === 'role.permissions.update' || item.action === 'role.data_scope.update'))
  const createRole = useMutation({ mutationFn: createPlatformRole, onSuccess: async () => { message.success('角色已创建'); setCreateOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色创建失败') })
  const editRoleMutation = useMutation({ mutationFn: (values: { name: string; description?: string; status: 'ACTIVE' | 'DISABLED' }) => updatePlatformRole(editRole!.key, values), onSuccess: async () => { message.success('角色信息已更新'); setEditRole(undefined); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色更新失败') })
  const roleStatusMutation = useMutation({ mutationFn: (role: PlatformRole) => updatePlatformRole(role.key, { name: role.name, description: role.description, status: role.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE' }), onSuccess: async (role) => { message.success(role.status === 'ACTIVE' ? '角色已启用' : '角色已停用'); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色状态更新失败') })
  const copyRoleMutation = useMutation({ mutationFn: (values: RoleForm) => copyPlatformRole(copyRole!.key, values), onSuccess: async () => { message.success('角色已复制'); setCopyRole(undefined); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色复制失败') })
  const columns: ProColumns<PlatformRole>[] = [
    { title: '角色', dataIndex: 'name', listSlot: 'title', render: (_, row) => <Typography.Text strong>{row.name}</Typography.Text> },
    { dataIndex: 'description', listSlot: 'description', search: false, render: (_, row) => <Typography.Text type="secondary">{row.description || '暂无说明'}</Typography.Text> },
    { title: '状态', dataIndex: 'status', listSlot: 'subTitle', valueType: 'select', valueEnum: { ACTIVE: { text: '启用', status: 'Success' }, DISABLED: { text: '停用', status: 'Default' } }, render: (_, row) => <Space size={4}><Tag color={row.status === 'ACTIVE' ? 'success' : 'default'}>{row.status === 'ACTIVE' ? '启用' : '停用'}</Tag><Tag>{row.key === 'system_admin' ? '全平台' : row.dataScope ? DATA_SCOPE_LABELS[row.dataScope] ?? '自定义范围' : '未设置'}</Tag></Space> },
    { dataIndex: 'permissions', listSlot: 'content', search: false, render: (_, row) => <Typography.Text>{row.key === 'system_admin' ? '拥有全部平台权限' : `${row.permissions.length} 项权限`}</Typography.Text> },
    { title: '可管理范围', dataIndex: 'dataScope', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(DATA_SCOPE_LABELS).map(([key, text]) => [key, { text }])) },
  ]
  if (canManage) columns.push({ title: '操作', listSlot: 'actions', valueType: 'option', search: false, render: (_, row) => [<Button key="permissions" type="link" onClick={() => setPermissionRole(row)}>权限</Button>, <Button key="scope" type="link" onClick={() => setScopeRole(row)}>管理范围</Button>, <Button key="edit" type="link" onClick={() => setEditRole(row)}>编辑</Button>, <Button key="copy" type="link" onClick={() => setCopyRole(row)}>复制</Button>, row.key !== 'system_admin' ? <Popconfirm key="status" title={row.status === 'ACTIVE' ? '停用此角色？' : '启用此角色？'} description={row.status === 'ACTIVE' ? '停用后，该角色不再授予权限。现有配置会保留。' : undefined} okText={row.status === 'ACTIVE' ? '停用' : '启用'} okButtonProps={{ danger: row.status === 'ACTIVE' }} onConfirm={() => roleStatusMutation.mutate(row)}><Button type="link" danger={row.status === 'ACTIVE'}>{row.status === 'ACTIVE' ? '停用' : '启用'}</Button></Popconfirm> : null].filter(Boolean) })

  return <PageContainer title="平台角色">
    {roles.isError ? <QueryErrorState refetch={roles.refetch} /> : <ProList<PlatformRole>
      className="velora-admin-primary-table velora-admin-entity-list"
      rowKey="key"
      columns={columns}
      dataSource={visibleRoles}
      loading={roles.isLoading}
      search={false}
      headerTitle={<AdminListScope value={scope} onChange={setScope} options={[
        { label: '全部角色', value: 'ALL', count: roleRows.length },
        { label: '启用', value: 'ACTIVE', count: roleRows.filter((item) => item.status === 'ACTIVE').length },
        { label: '停用', value: 'DISABLED', count: roleRows.filter((item) => item.status === 'DISABLED').length },
      ]} />}
      pagination={{ pageSize: 20, hideOnSinglePage: false }}
      toolBarRender={() => [<AdminListSearch key="search" value={searchValue} placeholder="搜索角色" onChange={setSearchValue} onSearch={setKeyword}>{canManage && <Button type="primary" onClick={() => setCreateOpen(true)}>新建角色</Button>}</AdminListSearch>]}
    />}
    {canManage && approved.length > 0 && <ProTable<ApprovalRequest> className="velora-admin-secondary-table" headerTitle="待执行变更" rowKey="id" dataSource={approved} search={false} pagination={{ pageSize: 20, hideOnSinglePage: false }} options={false} columns={[{ title: '事项', dataIndex: 'summary' }, { title: '操作', valueType: 'option', width: 100, render: (_, row) => <Button type="link" loading={execute.isPending} onClick={() => execute.mutate(row)}>执行变更</Button> }]} />}
    <ModalForm<{ permissions: string[]; approverId: string }> key={permissionRole?.key ?? 'permissions'} title={permissionRole ? `配置权限 · ${permissionRole.name}` : '配置权限'} open={Boolean(permissionRole)} onOpenChange={(value) => !value && setPermissionRole(undefined)} width={720} initialValues={{ permissions: permissionRole?.permissions ?? [] }} submitter={{ searchConfig: { submitText: '提交审批', resetText: '取消' } }} onFinish={async (values) => { await permissionRequest.mutateAsync(values); return true }}>
      <ProFormCheckbox.Group name="permissions" label="权限" options={(permissions.data ?? []).map((item) => ({ label: `${item.resource || '其他'} · ${item.name || item.description || item.key}`, value: item.key }))} />
      <ProForm.Item name="approverId" label="审批人" rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
    </ModalForm>
    <ModalForm<{ dataScope: string; departmentIds?: string[]; approverId: string }> key={scopeRole?.key ?? 'scope'} title={scopeRole ? `可管理范围 · ${scopeRole.name}` : '可管理范围'} open={Boolean(scopeRole)} onOpenChange={(value) => !value && setScopeRole(undefined)} width={560} initialValues={{ dataScope: scopeRole?.dataScope || 'SELF', departmentIds: scopeRole?.dataScopeDepartmentIds ?? [] }} submitter={{ searchConfig: { submitText: '提交审批', resetText: '取消' } }} onFinish={async (values) => { await scopeRequest.mutateAsync(values); return true }}>
      <ProFormRadio.Group name="dataScope" label="可管理的数据" options={Object.entries(DATA_SCOPE_LABELS).map(([value, label]) => ({ value, label }))} rules={[{ required: true }]} />
      <ProFormSelect name="departmentIds" label="指定部门" mode="multiple" options={(departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))} />
      <ProForm.Item name="approverId" label="审批人" rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
    </ModalForm>
    <ModalForm<RoleForm> title="新建角色" open={createOpen} onOpenChange={setCreateOpen} modalProps={{ destroyOnHidden: true }} submitter={{ searchConfig: { submitText: '创建', resetText: '取消' } }} onFinish={async (values) => { await createRole.mutateAsync(values); return true }}>
      <ProFormText name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]} />
      <ProFormText name="roleKey" label="角色标识" fieldProps={{ maxLength: 100 }} rules={[{ required: true, pattern: /^[a-z0-9_-]+$/, message: '仅支持小写字母、数字、下划线和短横线' }]} />
      <ProFormText name="description" label="说明" fieldProps={{ maxLength: 500 }} />
    </ModalForm>
    <ModalForm<{ name: string; description?: string; status: 'ACTIVE' | 'DISABLED' }> key={editRole?.key ?? 'edit'} title="编辑角色" open={Boolean(editRole)} onOpenChange={(open) => !open && setEditRole(undefined)} initialValues={{ name: editRole?.name, description: editRole?.description, status: editRole?.status ?? 'ACTIVE' }} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await editRoleMutation.mutateAsync(values); return true }}>
      <ProFormText name="name" label="角色名称" rules={[{ required: true }]} />
      <ProFormText name="description" label="说明" fieldProps={{ maxLength: 500 }} />
      <ProFormRadio.Group name="status" label="状态" disabled={editRole?.key === 'system_admin'} options={[{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'DISABLED' }]} />
    </ModalForm>
    <ModalForm<RoleForm> key={copyRole?.key ?? 'copy'} title="复制角色" open={Boolean(copyRole)} onOpenChange={(open) => !open && setCopyRole(undefined)} initialValues={{ name: copyRole ? `${copyRole.name}副本` : '', description: copyRole?.description }} submitter={{ searchConfig: { submitText: '复制', resetText: '取消' } }} onFinish={async (values) => { await copyRoleMutation.mutateAsync(values); return true }}>
      <ProFormText name="name" label="角色名称" rules={[{ required: true }]} />
      <ProFormText name="roleKey" label="角色标识" rules={[{ required: true, pattern: /^[a-z0-9_-]+$/, message: '仅支持小写字母、数字、下划线和短横线' }]} />
      <ProFormText name="description" label="说明" fieldProps={{ maxLength: 500 }} />
    </ModalForm>
  </PageContainer>
}

function normalized(values: string[]): string[] { return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean))).sort() }
