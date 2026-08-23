import { useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { ModalForm, PageContainer, ProFormCheckbox, ProFormRadio, ProFormSelect, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { listDepartments, listPlatformPermissions, listPlatformRoles, updateRoleDataScope, updateRolePermissions } from '../../api/admin-platform'
import type { PlatformRole } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'

const DATA_SCOPE_LABELS: Record<string, string> = { ALL: '全部数据', DEPARTMENT: '指定部门', SELF_DEPARTMENT: '本部门', SELF: '仅本人' }

export default function Roles() {
  usePageTitle('平台角色')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [permissionRole, setPermissionRole] = useState<PlatformRole>()
  const [scopeRole, setScopeRole] = useState<PlatformRole>()
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const permissions = useQuery({ queryKey: ['admin', 'permissions'], queryFn: listPlatformPermissions })
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'roles'] })
  const permissionMutation = useMutation({ mutationFn: (values: { permissions: string[] }) => updateRolePermissions(permissionRole!.key, values.permissions ?? []), onSuccess: async () => { message.success('角色权限已更新'); setPermissionRole(undefined); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '权限保存失败') })
  const scopeMutation = useMutation({ mutationFn: (values: { dataScope: string; departmentIds?: string[] }) => updateRoleDataScope(scopeRole!.key, values.dataScope, values.departmentIds ?? []), onSuccess: async () => { message.success('数据范围已更新'); setScopeRole(undefined); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '数据范围保存失败') })
  const columns: ProColumns<PlatformRole>[] = [
    { title: '角色', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.description || '—'}</Typography.Text></Space> },
    { title: '权限数', dataIndex: 'permissions', search: false, width: 100, render: (_, row) => `${row.permissions.length} 项` },
    { title: '数据范围', dataIndex: 'dataScope', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(DATA_SCOPE_LABELS).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag>{DATA_SCOPE_LABELS[row.dataScope] ?? '自定义'}</Tag> },
    { title: '操作', valueType: 'option', width: 180, render: (_, row) => <Space><Button type="link" onClick={() => setPermissionRole(row)}>配置权限</Button><Button type="link" onClick={() => setScopeRole(row)}>数据范围</Button></Space> },
  ]
  const permissionGroups = Object.entries((permissions.data ?? []).reduce<Record<string, typeof permissions.data>>((groups, item) => {
    const resource = item.resource || '其他'
    groups[resource] = [...(groups[resource] ?? []), item]
    return groups
  }, {}))

  return <PageContainer title="平台角色">
    <ProTable<PlatformRole> rowKey="key" columns={columns} dataSource={roles.data ?? []} loading={roles.isLoading} search={{ labelWidth: 'auto' }} pagination={false} options={{ density: false }} />
    <ModalForm<{ permissions: string[] }> key={permissionRole?.key ?? 'permissions'} title={permissionRole ? `配置权限 · ${permissionRole.name}` : '配置权限'} open={Boolean(permissionRole)} onOpenChange={(value) => !value && setPermissionRole(undefined)} width={720} initialValues={{ permissions: permissionRole?.permissions ?? [] }} submitter={{ searchConfig: { submitText: '保存权限', resetText: '取消' } }} onFinish={async (values) => { await permissionMutation.mutateAsync(values); return true }}>
      {permissionGroups.map(([resource, items]) => <ProFormCheckbox.Group key={resource} name="permissions" label={resource} options={(items ?? []).map((item) => ({ label: item.name || item.description || item.key, value: item.key }))} />)}
    </ModalForm>
    <ModalForm<{ dataScope: string; departmentIds?: string[] }> key={scopeRole?.key ?? 'scope'} title={scopeRole ? `数据范围 · ${scopeRole.name}` : '数据范围'} open={Boolean(scopeRole)} onOpenChange={(value) => !value && setScopeRole(undefined)} width={560} initialValues={{ dataScope: scopeRole?.dataScope || 'SELF', departmentIds: scopeRole?.dataScopeDepartmentIds ?? [] }} submitter={{ searchConfig: { submitText: '保存范围', resetText: '取消' } }} onFinish={async (values) => { await scopeMutation.mutateAsync(values); return true }}>
      <ProFormRadio.Group name="dataScope" label="可管理的数据" options={Object.entries(DATA_SCOPE_LABELS).map(([value, label]) => ({ value, label }))} rules={[{ required: true }]} />
      <ProFormSelect name="departmentIds" label="指定部门" mode="multiple" options={(departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))} />
    </ModalForm>
  </PageContainer>
}
