import { useMemo, useState } from 'react'
import { App, Button, Popconfirm, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { DrawerForm, PageContainer, ProDescriptions, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createDepartment, createPosition, getOrganization, listDepartments, listPositions, updateDepartment, updateOrganization, updatePosition, type DepartmentInput, type OrganizationInput, type PositionInput } from '../../api/admin-platform'
import type { Department, Position } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { SYSTEM_DEPARTMENT_MANAGE, SYSTEM_DEPARTMENT_READ, SYSTEM_ORGANIZATION_MANAGE, SYSTEM_POSITION_MANAGE, SYSTEM_POSITION_READ } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import QueryErrorState from '../../components/QueryErrorState'
import { AdminListScope, AdminListSearch } from '../../components/admin/AdminListToolbar'

const ACTIVE_OPTIONS = [{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'DISABLED' }]

function statusTag(status: string) {
  return status === 'ACTIVE' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>
}

export default function Organization() {
  usePageTitle('组织架构')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const canManageDepartments = useAdminPermission(SYSTEM_DEPARTMENT_MANAGE)
  const canManagePositions = useAdminPermission(SYSTEM_POSITION_MANAGE)
  const canManageOrganization = useAdminPermission(SYSTEM_ORGANIZATION_MANAGE)
  const canReadDepartments = useAdminPermission(SYSTEM_DEPARTMENT_READ)
  const canReadPositions = useAdminPermission(SYSTEM_POSITION_READ)
  const [tab, setTab] = useState('organization')
  const [organizationOpen, setOrganizationOpen] = useState(false)
  const [departmentOpen, setDepartmentOpen] = useState(false)
  const [positionOpen, setPositionOpen] = useState(false)
  const [editingDepartment, setEditingDepartment] = useState<Department>()
  const [editingPosition, setEditingPosition] = useState<Position>()
  const [departmentScope, setDepartmentScope] = useState('ALL')
  const [departmentSearch, setDepartmentSearch] = useState('')
  const [departmentKeyword, setDepartmentKeyword] = useState('')
  const [positionScope, setPositionScope] = useState('ALL')
  const [positionSearch, setPositionSearch] = useState('')
  const [positionKeyword, setPositionKeyword] = useState('')
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments, enabled: canReadDepartments || canReadPositions })
  const positions = useQuery({ queryKey: ['admin', 'positions'], queryFn: listPositions, enabled: canReadPositions })
  const organization = useQuery({ queryKey: ['admin', 'organization'], queryFn: getOrganization })
  const allDepartments = departments.data ?? []
  const allPositions = positions.data ?? []
  const visibleDepartments = useMemo(() => allDepartments.filter((item) => (departmentScope === 'ALL' || item.status === departmentScope) && (!departmentKeyword || `${item.name} ${item.departmentKey}`.toLowerCase().includes(departmentKeyword.toLowerCase()))), [allDepartments, departmentKeyword, departmentScope])
  const visiblePositions = useMemo(() => allPositions.filter((item) => (positionScope === 'ALL' || item.status === positionScope) && (!positionKeyword || `${item.name} ${item.positionKey} ${item.description ?? ''}`.toLowerCase().includes(positionKeyword.toLowerCase()))), [allPositions, positionKeyword, positionScope])
  const refresh = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'departments'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'positions'] })]) }

  const departmentMutation = useMutation({
    mutationFn: (values: DepartmentInput) => editingDepartment ? updateDepartment(editingDepartment.id, values) : createDepartment(values),
    onSuccess: async () => { message.success(editingDepartment ? '部门已更新' : '部门已创建'); setDepartmentOpen(false); await refresh() },
    onError: (error) => message.error(error instanceof Error ? error.message : '部门保存失败'),
  })
  const positionMutation = useMutation({
    mutationFn: (values: PositionInput) => editingPosition ? updatePosition(editingPosition.id, values) : createPosition(values),
    onSuccess: async () => { message.success(editingPosition ? '岗位已更新' : '岗位已创建'); setPositionOpen(false); await refresh() },
    onError: (error) => message.error(error instanceof Error ? error.message : '岗位保存失败'),
  })
  const organizationMutation = useMutation({ mutationFn: updateOrganization, onSuccess: async () => { message.success('组织信息已更新'); setOrganizationOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'organization'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '组织信息保存失败') })
  const departmentStatusMutation = useMutation({ mutationFn: (department: Department) => updateDepartment(department.id, { name: department.name, parentId: department.parentId, status: department.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', sortOrder: department.sortOrder }), onSuccess: async (department) => { message.success(department.status === 'ACTIVE' ? '部门已启用' : '部门已停用'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '部门状态更新失败') })
  const positionStatusMutation = useMutation({ mutationFn: (position: Position) => updatePosition(position.id, { name: position.name, description: position.description, departmentId: position.departmentId, status: position.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE', sortOrder: position.sortOrder }), onSuccess: async (position) => { message.success(position.status === 'ACTIVE' ? '岗位已启用' : '岗位已停用'); await refresh() }, onError: (error) => message.error(error instanceof Error ? error.message : '岗位状态更新失败') })

  const departmentById = new Map((departments.data ?? []).map((item) => [item.id, item]))
  const departmentColumns: ProColumns<Department>[] = [
    { title: '部门', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.departmentKey}</Typography.Text></Space> },
    { title: '上级部门', dataIndex: 'parentId', search: false, render: (_, row) => row.parentId ? departmentById.get(row.parentId)?.name ?? '—' : '根部门' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } }, render: (_, row) => statusTag(row.status) },
    { title: '排序', dataIndex: 'sortOrder', search: false, width: 90 },
  ]
  if (canManageDepartments) departmentColumns.push({ title: '操作', valueType: 'option', width: 150, render: (_, row) => <Space className="table-action-cell"><Button type="link" onClick={() => { setEditingDepartment(row); setDepartmentOpen(true) }}>编辑</Button><Popconfirm title={row.status === 'ACTIVE' ? '停用此部门？' : '启用此部门？'} description={row.status === 'ACTIVE' ? '停用后不能再用于新增人员和岗位。已有数据会保留。' : undefined} okText={row.status === 'ACTIVE' ? '停用' : '启用'} okButtonProps={{ danger: row.status === 'ACTIVE' }} onConfirm={() => departmentStatusMutation.mutate(row)}><Button type="link" danger={row.status === 'ACTIVE'}>{row.status === 'ACTIVE' ? '停用' : '启用'}</Button></Popconfirm></Space> })
  const positionColumns: ProColumns<Position>[] = [
    { title: '岗位', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.positionKey}</Typography.Text></Space> },
    { title: '所属部门', dataIndex: 'departmentId', valueType: 'select', valueEnum: Object.fromEntries((departments.data ?? []).map((item) => [item.id, item.name])), render: (_, row) => departmentById.get(row.departmentId)?.name ?? '—' },
    { title: '说明', dataIndex: 'description', search: false, ellipsis: true },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } }, render: (_, row) => statusTag(row.status) },
  ]
  if (canManagePositions) positionColumns.push({ title: '操作', valueType: 'option', width: 150, render: (_, row) => <Space className="table-action-cell"><Button type="link" onClick={() => { setEditingPosition(row); setPositionOpen(true) }}>编辑</Button><Popconfirm title={row.status === 'ACTIVE' ? '停用此岗位？' : '启用此岗位？'} description={row.status === 'ACTIVE' ? '停用后不能再分配给人员。已有任职记录会保留。' : undefined} okText={row.status === 'ACTIVE' ? '停用' : '启用'} okButtonProps={{ danger: row.status === 'ACTIVE' }} onConfirm={() => positionStatusMutation.mutate(row)}><Button type="link" danger={row.status === 'ACTIVE'}>{row.status === 'ACTIVE' ? '停用' : '启用'}</Button></Popconfirm></Space> })

  const tabs = [{ key: 'organization', tab: '组织信息' }, ...(canReadDepartments ? [{ key: 'departments', tab: '部门' }] : []), ...(canReadPositions ? [{ key: 'positions', tab: '岗位' }] : [])]
  return <PageContainer title="组织架构" tabList={tabs} tabActiveKey={tab} onTabChange={setTab} extra={tab === 'organization' && canManageOrganization ? <Button type="primary" onClick={() => setOrganizationOpen(true)}>编辑组织信息</Button> : undefined}>
    {tab === 'organization' && (organization.isError ? <QueryErrorState refetch={organization.refetch} /> : <ProDescriptions className="velora-admin-page-card" loading={organization.isLoading} column={2} dataSource={organization.data} columns={[
      { title: '组织名称', dataIndex: 'name' },
      { title: '组织编码', dataIndex: 'organizationKey' },
      { title: '状态', dataIndex: 'status', render: (_, row) => statusTag(row.status) },
      { title: '用户上限', dataIndex: 'maxUsers' },
      { title: '每人会话上限', dataIndex: 'maxActiveSessions' },
      { title: '说明', dataIndex: 'description', span: 2, render: (_, row) => row.description || '—' },
    ]} />)}
    {tab === 'departments' && (departments.isError ? <QueryErrorState refetch={departments.refetch} /> : <ProTable<Department> className="velora-admin-primary-table" rowKey="id" loading={departments.isLoading} dataSource={visibleDepartments} columns={departmentColumns} search={false} pagination={{ pageSize: 20, hideOnSinglePage: false }} headerTitle={<AdminListScope value={departmentScope} onChange={setDepartmentScope} options={[{ label: '全部部门', value: 'ALL', count: allDepartments.length }, { label: '启用', value: 'ACTIVE', count: allDepartments.filter((item) => item.status === 'ACTIVE').length }, { label: '停用', value: 'DISABLED', count: allDepartments.filter((item) => item.status === 'DISABLED').length }]} />} toolBarRender={() => [<AdminListSearch key="tools" value={departmentSearch} placeholder="搜索部门名称或编码" onChange={setDepartmentSearch} onSearch={setDepartmentKeyword}>{canManageDepartments && <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingDepartment(undefined); setDepartmentOpen(true) }}>新建部门</Button>}</AdminListSearch>]} />)}
    {tab === 'positions' && (positions.isError || departments.isError ? <QueryErrorState refetch={() => Promise.all([positions.refetch(), departments.refetch()])} /> : <ProTable<Position> className="velora-admin-primary-table" rowKey="id" loading={positions.isLoading || departments.isLoading} dataSource={visiblePositions} columns={positionColumns} search={false} pagination={{ pageSize: 20, hideOnSinglePage: false }} headerTitle={<AdminListScope value={positionScope} onChange={setPositionScope} options={[{ label: '全部岗位', value: 'ALL', count: allPositions.length }, { label: '启用', value: 'ACTIVE', count: allPositions.filter((item) => item.status === 'ACTIVE').length }, { label: '停用', value: 'DISABLED', count: allPositions.filter((item) => item.status === 'DISABLED').length }]} />} toolBarRender={() => [<AdminListSearch key="tools" value={positionSearch} placeholder="搜索岗位名称或编码" onChange={setPositionSearch} onSearch={setPositionKeyword}>{canManagePositions && <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingPosition(undefined); setPositionOpen(true) }}>新建岗位</Button>}</AdminListSearch>]} />)}

    <DrawerForm<OrganizationInput> key={organization.data?.updatedAt ?? 'organization'} title="编辑组织信息" open={organizationOpen} onOpenChange={setOrganizationOpen} width={520} initialValues={organization.data} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await organizationMutation.mutateAsync(values); return true }}>
      <ProFormText name="name" label="组织名称" rules={[{ required: true, message: '请输入组织名称' }]} />
      <ProFormTextArea name="description" label="说明" fieldProps={{ maxLength: 500, showCount: true }} />
      <ProFormSelect name="status" label="状态" options={ACTIVE_OPTIONS} rules={[{ required: true }]} />
      <ProFormDigit name="maxUsers" label="用户上限" min={1} fieldProps={{ precision: 0 }} rules={[{ required: true, message: '请输入用户上限' }]} />
      <ProFormDigit name="maxActiveSessions" label="每人会话上限" min={1} max={100} fieldProps={{ precision: 0 }} rules={[{ required: true, message: '请输入会话上限' }]} />
    </DrawerForm>

    <DrawerForm<DepartmentInput> key={editingDepartment?.id ?? 'new-department'} title={editingDepartment ? '编辑部门' : '新建部门'} open={departmentOpen} onOpenChange={setDepartmentOpen} width={520} initialValues={editingDepartment ?? { status: 'ACTIVE', sortOrder: 0, parentId: '' }} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await departmentMutation.mutateAsync(values); return true }}>
      {!editingDepartment && <ProFormText name="departmentKey" label="部门编码" rules={[{ required: true, message: '请输入部门编码' }, { pattern: /^[a-z][a-z0-9_-]{1,63}$/, message: '使用小写字母、数字、短横线或下划线' }]} />}
      <ProFormText name="name" label="部门名称" rules={[{ required: true, message: '请输入部门名称' }]} />
      <ProFormSelect name="parentId" label="上级部门" options={[{ label: '无（根部门）', value: '' }, ...(departments.data ?? []).filter((item) => item.id !== editingDepartment?.id).map((item) => ({ label: item.name, value: item.id }))]} />
      <ProFormSelect name="status" label="状态" options={ACTIVE_OPTIONS} rules={[{ required: true }]} />
      <ProFormDigit name="sortOrder" label="排序" min={0} fieldProps={{ precision: 0 }} />
    </DrawerForm>

    <DrawerForm<PositionInput> key={editingPosition?.id ?? 'new-position'} title={editingPosition ? '编辑岗位' : '新建岗位'} open={positionOpen} onOpenChange={setPositionOpen} width={520} initialValues={editingPosition ?? { status: 'ACTIVE', sortOrder: 0 }} submitter={{ searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await positionMutation.mutateAsync(values); return true }}>
      {!editingPosition && <ProFormText name="positionKey" label="岗位编码" rules={[{ required: true, message: '请输入岗位编码' }, { pattern: /^[a-z][a-z0-9_-]{1,63}$/, message: '使用小写字母、数字、短横线或下划线' }]} />}
      <ProFormText name="name" label="岗位名称" rules={[{ required: true, message: '请输入岗位名称' }]} />
      <ProFormSelect name="departmentId" label="所属部门" options={(departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))} rules={[{ required: true, message: '请选择所属部门' }]} />
      <ProFormText name="description" label="岗位说明" />
      <ProFormSelect name="status" label="状态" options={ACTIVE_OPTIONS} rules={[{ required: true }]} />
      <ProFormDigit name="sortOrder" label="排序" min={0} fieldProps={{ precision: 0 }} />
    </DrawerForm>
  </PageContainer>
}
