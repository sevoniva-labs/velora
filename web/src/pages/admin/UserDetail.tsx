import { useMemo, useState } from 'react'
import { App, Button, Empty, Space, Tag, Typography } from 'antd'
import { DrawerForm, ModalForm, PageContainer, ProDescriptions, ProFormList, ProFormSelect, ProFormSwitch, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { adminListApplications, adminListUsers } from '../../api/api'
import { listDepartments, listPlatformRoles, listPositions, listUserAssignments, replaceUserAssignments, updateUserRoles } from '../../api/admin-platform'
import type { ApplicationEntitlement, UserAssignment } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'

interface AssignmentForm { assignments: UserAssignment[] }
interface RoleForm { roles: string[] }

export default function UserDetail() {
  const { id = '' } = useParams()
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [tab, setTab] = useState('profile')
  const [assignmentOpen, setAssignmentOpen] = useState(false)
  const [roleOpen, setRoleOpen] = useState(false)
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: adminListUsers })
  const user = users.data?.find((item) => item.id === id)
  usePageTitle(user?.displayName || '用户详情')
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const positions = useQuery({ queryKey: ['admin', 'positions'], queryFn: listPositions })
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const applications = useQuery({ queryKey: ['admin', 'applications', 'user-detail'], queryFn: () => adminListApplications({ page: 1, pageSize: 500 }) })
  const assignments = useQuery({ queryKey: ['admin', 'users', id, 'assignments'], queryFn: () => listUserAssignments(id), enabled: Boolean(id) })
  const departmentNames = useMemo(() => new Map((departments.data ?? []).map((item) => [item.id, item.name])), [departments.data])
  const positionNames = useMemo(() => new Map((positions.data ?? []).map((item) => [item.id, item.name])), [positions.data])
  const applicationNames = useMemo(() => new Map((applications.data?.items ?? []).map((item) => [item.code, item.name])), [applications.data])
  const assignmentMutation = useMutation({ mutationFn: (values: AssignmentForm) => replaceUserAssignments(id, values.assignments ?? []), onSuccess: async () => { message.success('任职信息已更新'); setAssignmentOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'users', id, 'assignments'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '任职信息保存失败') })
  const roleMutation = useMutation({ mutationFn: (values: RoleForm) => updateUserRoles(id, values.roles ?? []), onSuccess: async () => { message.success('平台角色已更新'); setRoleOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '平台角色保存失败') })

  if (!users.isLoading && !user) return <PageContainer title="用户详情"><Empty description="用户不存在或已删除" /></PageContainer>
  const assignmentColumns: ProColumns<UserAssignment>[] = [
    { title: '部门', dataIndex: 'departmentId', render: (_, row) => departmentNames.get(row.departmentId) ?? '—' },
    { title: '岗位', dataIndex: 'positionId', render: (_, row) => row.positionId ? positionNames.get(row.positionId) ?? '—' : '—' },
    { title: '任职类型', dataIndex: 'primary', render: (_, row) => row.primary ? <Tag color="blue">主职</Tag> : <Tag>兼任</Tag> },
  ]
  const entitlementColumns: ProColumns<ApplicationEntitlement>[] = [
    { title: '应用', dataIndex: 'applicationCode', render: (_, row) => <Space direction="vertical" size={0}><Typography.Text strong>{applicationNames.get(row.applicationCode) ?? row.applicationCode}</Typography.Text><Typography.Text type="secondary">{row.applicationCode}</Typography.Text></Space> },
    { title: '应用角色', dataIndex: 'roles', render: (_, row) => row.roles.length ? row.roles.map((role) => <Tag key={role}>{role}</Tag>) : '仅访问' },
    { title: '状态', dataIndex: 'status', render: (_, row) => row.status === 'ACTIVE' ? <Tag color="success">生效中</Tag> : <Tag>已停用</Tag> },
    { title: '来源', render: () => <Typography.Text type="secondary">历史直接授权</Typography.Text> },
  ]

  return <PageContainer title={user?.displayName || user?.loginName || '用户详情'} onBack={() => history.back()} tabList={[{ key: 'profile', tab: '基本信息' }, { key: 'assignments', tab: '部门与岗位' }, { key: 'access', tab: '有效应用权限' }]} tabActiveKey={tab} onTabChange={setTab} extra={tab === 'profile' ? [<Button key="roles" type="primary" onClick={() => setRoleOpen(true)}>配置平台角色</Button>] : tab === 'assignments' ? [<Button key="assignments" type="primary" onClick={() => setAssignmentOpen(true)}>编辑任职</Button>] : undefined}>
    {tab === 'profile' && <ProDescriptions column={2} dataSource={user} columns={[
      { title: '登录账号', dataIndex: 'loginName' },
      { title: '姓名', dataIndex: 'displayName' },
      { title: '邮箱', dataIndex: 'email', render: (_, row) => row.email || '—' },
      { title: '账号状态', dataIndex: 'status', render: (_, row) => row.status === 'ACTIVE' ? <Tag color="success">正常</Tag> : row.status === 'LOCKED' ? <Tag color="warning">已锁定</Tag> : <Tag>已停用</Tag> },
      { title: '平台角色', dataIndex: 'roles', render: (_, row) => row.roles.map((key) => <Tag key={key}>{roles.data?.find((role) => role.key === key)?.name ?? key}</Tag>) },
      { title: '身份来源', dataIndex: 'identitySource', render: () => '统一身份' },
    ]} />}
    {tab === 'assignments' && <ProTable<UserAssignment> rowKey={(row) => row.id ?? `${row.departmentId}-${row.positionId}`} columns={assignmentColumns} dataSource={assignments.data ?? []} loading={assignments.isLoading} search={false} pagination={false} options={false} />}
    {tab === 'access' && <ProTable<ApplicationEntitlement> rowKey="applicationCode" columns={entitlementColumns} dataSource={user?.entitlements ?? []} search={false} pagination={false} options={false} locale={{ emptyText: '暂无应用权限' }} />}

    <DrawerForm<AssignmentForm> key={`${id}-${assignments.data?.length ?? 0}`} title="编辑任职" open={assignmentOpen} onOpenChange={setAssignmentOpen} width={620} initialValues={{ assignments: assignments.data ?? [] }} submitter={{ searchConfig: { submitText: '保存任职', resetText: '取消' } }} onFinish={async (values) => { const rows = values.assignments ?? []; if (rows.length && rows.filter((item) => item.primary).length !== 1) throw new Error('请设置且仅设置一个主职'); await assignmentMutation.mutateAsync(values); return true }}>
      <ProFormList name="assignments" creatorButtonProps={{ creatorButtonText: '添加任职' }} copyIconProps={false}>
        <ProFormSelect name="departmentId" label="部门" options={(departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))} rules={[{ required: true, message: '请选择部门' }]} />
        <ProFormSelect name="positionId" label="岗位" options={(positions.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))} />
        <ProFormSwitch name="primary" label="主职" />
      </ProFormList>
    </DrawerForm>
    <ModalForm<RoleForm> key={`${id}-${user?.roles.join('-')}`} title="配置平台角色" open={roleOpen} onOpenChange={setRoleOpen} initialValues={{ roles: user?.roles ?? [] }} submitter={{ searchConfig: { submitText: '保存角色', resetText: '取消' } }} onFinish={async (values) => { await roleMutation.mutateAsync(values); return true }}>
      <ProFormSelect name="roles" label="平台角色" mode="multiple" options={(roles.data ?? []).map((role) => ({ label: role.name, value: role.key }))} rules={[{ required: true, message: '至少保留一个平台角色' }]} />
    </ModalForm>
  </PageContainer>
}
