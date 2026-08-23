import { useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { DrawerForm, PageContainer, ProFormDigit, ProFormSelect, ProFormText, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createDepartment, createPosition, listDepartments, listPositions, updateDepartment, updatePosition, type DepartmentInput, type PositionInput } from '../../api/admin-platform'
import type { Department, Position } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'

const ACTIVE_OPTIONS = [{ label: '启用', value: 'ACTIVE' }, { label: '停用', value: 'DISABLED' }]

function statusTag(status: string) {
  return status === 'ACTIVE' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag>
}

export default function Organization() {
  usePageTitle('部门与岗位')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState('departments')
  const [departmentOpen, setDepartmentOpen] = useState(false)
  const [positionOpen, setPositionOpen] = useState(false)
  const [editingDepartment, setEditingDepartment] = useState<Department>()
  const [editingPosition, setEditingPosition] = useState<Position>()
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const positions = useQuery({ queryKey: ['admin', 'positions'], queryFn: listPositions })
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

  const departmentById = new Map((departments.data ?? []).map((item) => [item.id, item]))
  const departmentColumns: ProColumns<Department>[] = [
    { title: '部门', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.departmentKey}</Typography.Text></Space> },
    { title: '上级部门', dataIndex: 'parentId', search: false, render: (_, row) => row.parentId ? departmentById.get(row.parentId)?.name ?? '—' : '根部门' },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } }, render: (_, row) => statusTag(row.status) },
    { title: '排序', dataIndex: 'sortOrder', search: false, width: 90 },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => <Button type="link" onClick={() => { setEditingDepartment(row); setDepartmentOpen(true) }}>编辑</Button> },
  ]
  const positionColumns: ProColumns<Position>[] = [
    { title: '岗位', dataIndex: 'name', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{row.name}</Typography.Text><Typography.Text type="secondary">{row.positionKey}</Typography.Text></Space> },
    { title: '所属部门', dataIndex: 'departmentId', valueType: 'select', valueEnum: Object.fromEntries((departments.data ?? []).map((item) => [item.id, item.name])), render: (_, row) => departmentById.get(row.departmentId)?.name ?? '—' },
    { title: '说明', dataIndex: 'description', search: false, ellipsis: true },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { ACTIVE: { text: '启用' }, DISABLED: { text: '停用' } }, render: (_, row) => statusTag(row.status) },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => <Button type="link" onClick={() => { setEditingPosition(row); setPositionOpen(true) }}>编辑</Button> },
  ]

  return <PageContainer title="部门与岗位" tabList={[{ key: 'departments', tab: '部门' }, { key: 'positions', tab: '岗位' }]} tabActiveKey={tab} onTabChange={setTab}>
    {tab === 'departments' ? <ProTable<Department> rowKey="id" loading={departments.isLoading} dataSource={departments.data ?? []} columns={departmentColumns} search={{ labelWidth: 'auto' }} pagination={false} toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => { setEditingDepartment(undefined); setDepartmentOpen(true) }}>新建部门</Button>]} /> : <ProTable<Position> rowKey="id" loading={positions.isLoading || departments.isLoading} dataSource={positions.data ?? []} columns={positionColumns} search={{ labelWidth: 'auto' }} pagination={false} toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => { setEditingPosition(undefined); setPositionOpen(true) }}>新建岗位</Button>]} />}

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
