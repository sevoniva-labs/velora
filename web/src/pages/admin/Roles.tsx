import { useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { ModalForm, PageContainer, ProForm, ProFormCheckbox, ProFormRadio, ProFormSelect, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { copyPlatformRole, createApproval, createPlatformRole, listApprovals, listDepartments, listPlatformPermissions, listPlatformRoles, updatePlatformRole, updateRoleDataScope, updateRolePermissions } from '../../api/admin-platform'
import type { ApprovalRequest, PlatformRole } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { useMe } from '../../auth/useMe'
import AdminUserSelect from '../../components/AdminUserSelect'
import { SYSTEM_ROLE_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'

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
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const permissions = useQuery({ queryKey: ['admin', 'permissions'], queryFn: listPlatformPermissions })
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const approvals = useQuery({ queryKey: ['admin', 'approvals'], queryFn: listApprovals, enabled: canManage })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'roles'] })
  const refreshAll = async () => { await Promise.all([refresh(), queryClient.invalidateQueries({ queryKey: ['admin', 'approvals'] })]) }
  const permissionRequest = useMutation({ mutationFn: (values: { permissions: string[]; approverId: string }) => { const permissions = normalized(values.permissions); return createApproval({ requestType: 'ROLE_PERMISSION_CHANGE', action: 'role.permissions.update', resource: 'role', resourceId: permissionRole!.key, summary: `更新平台角色“${permissionRole!.name}”的权限`, payloadJson: JSON.stringify({ permissions }), approverIds: [values.approverId] }) }, onSuccess: async () => { message.success('角色权限变更已提交审批'); setPermissionRole(undefined); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '审批提交失败') })
  const scopeRequest = useMutation({ mutationFn: (values: { dataScope: string; departmentIds?: string[]; approverId: string }) => { const departmentIds = normalized(values.departmentIds ?? []); return createApproval({ requestType: 'ROLE_DATA_SCOPE_CHANGE', action: 'role.data_scope.update', resource: 'role', resourceId: scopeRole!.key, summary: `更新平台角色“${scopeRole!.name}”的数据范围`, payloadJson: JSON.stringify({ data_scope: values.dataScope.trim(), department_ids: departmentIds }), approverIds: [values.approverId] }) }, onSuccess: async () => { message.success('数据范围变更已提交审批'); setScopeRole(undefined); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '审批提交失败') })
  const execute = useMutation({ mutationFn: async (approval: ApprovalRequest) => { const payload = JSON.parse(approval.payloadJson) as { permissions?: string[]; data_scope?: string; department_ids?: string[] }; if (approval.action === 'role.permissions.update') return updateRolePermissions(approval.resourceId, payload.permissions ?? [], approval.id); return updateRoleDataScope(approval.resourceId, payload.data_scope ?? 'SELF', payload.department_ids ?? [], approval.id) }, onSuccess: async () => { message.success('角色变更已生效'); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色变更执行失败') })
  const approved = (approvals.data ?? []).filter((item) => item.resource === 'role' && item.status === 'APPROVED' && (item.action === 'role.permissions.update' || item.action === 'role.data_scope.update'))
  const createRole = useMutation({ mutationFn: createPlatformRole, onSuccess: async () => { message.success('角色已创建'); setCreateOpen(false); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色创建失败') })
  const editRoleMutation = useMutation({ mutationFn: (values: { name: string; description?: string; status: 'ACTIVE' | 'DISABLED' }) => updatePlatformRole(editRole!.key, values), onSuccess: async () => { message.success('角色信息已更新'); setEditRole(undefined); await refreshAll() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色更新失败') })
  const copyRoleMutation = useMutation({ mutationFn: (values: RoleForm) => copyPlatformRole(copyRole!.key, values), onSuccess: async () => { message.success('角色已复制'); setCopyRole(undefined); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '角色复制失败') })
  const columns: ProColumns<PlatformRole>[] = [
    { title: '角色', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.description || '—'}</Typography.Text></Space> },
    { title: '权限数', dataIndex: 'permissions', search: false, width: 100, render: (_, row) => `${row.permissions.length} 项` },
    { title: '数据范围', dataIndex: 'dataScope', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(DATA_SCOPE_LABELS).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag>{DATA_SCOPE_LABELS[row.dataScope] ?? '自定义'}</Tag> },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用', status: 'Success' }, DISABLED: { text: '停用', status: 'Default' } }, width: 90 },
  ]
  if (canManage) columns.push({ title: '操作', valueType: 'option', width: 320, render: (_, row) => <Space className="table-action-cell"><Button type="link" onClick={() => setPermissionRole(row)}>配置权限</Button><Button type="link" onClick={() => setScopeRole(row)}>数据范围</Button><Button type="link" onClick={() => setEditRole(row)}>编辑</Button><Button type="link" onClick={() => setCopyRole(row)}>复制</Button></Space> })

  return <PageContainer title="平台角色">
    <ProTable<PlatformRole> rowKey="key" columns={columns} dataSource={roles.data ?? []} loading={roles.isLoading} search={{ labelWidth: 'auto' }} pagination={false} options={{ density: false }} toolBarRender={canManage ? () => [<Button key="create" type="primary" onClick={() => setCreateOpen(true)}>新建角色</Button>] : false} />
    {canManage && approved.length > 0 && <ProTable<ApprovalRequest> headerTitle="待执行变更" rowKey="id" dataSource={approved} search={false} pagination={false} options={false} columns={[{ title: '事项', dataIndex: 'summary' }, { title: '操作', valueType: 'option', width: 100, render: (_, row) => <Button type="link" loading={execute.isPending} onClick={() => execute.mutate(row)}>执行变更</Button> }]} />}
    <ModalForm<{ permissions: string[]; approverId: string }> key={permissionRole?.key ?? 'permissions'} title={permissionRole ? `配置权限 · ${permissionRole.name}` : '配置权限'} open={Boolean(permissionRole)} onOpenChange={(value) => !value && setPermissionRole(undefined)} width={720} initialValues={{ permissions: permissionRole?.permissions ?? [] }} submitter={{ searchConfig: { submitText: '提交审批', resetText: '取消' } }} onFinish={async (values) => { await permissionRequest.mutateAsync(values); return true }}>
      <ProFormCheckbox.Group name="permissions" label="权限" options={(permissions.data ?? []).map((item) => ({ label: `${item.resource || '其他'} · ${item.name || item.description || item.key}`, value: item.key }))} />
      <ProForm.Item name="approverId" label="审批人" rules={[{ required: true, message: '请选择审批人' }]}><AdminUserSelect excludeIds={me.data?.id ? [me.data.id] : []} /></ProForm.Item>
    </ModalForm>
    <ModalForm<{ dataScope: string; departmentIds?: string[]; approverId: string }> key={scopeRole?.key ?? 'scope'} title={scopeRole ? `数据范围 · ${scopeRole.name}` : '数据范围'} open={Boolean(scopeRole)} onOpenChange={(value) => !value && setScopeRole(undefined)} width={560} initialValues={{ dataScope: scopeRole?.dataScope || 'SELF', departmentIds: scopeRole?.dataScopeDepartmentIds ?? [] }} submitter={{ searchConfig: { submitText: '提交审批', resetText: '取消' } }} onFinish={async (values) => { await scopeRequest.mutateAsync(values); return true }}>
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
