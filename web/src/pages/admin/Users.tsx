import { useMemo, useState } from 'react'
import { App as AntdApp, Button, Checkbox, Form, Input, Modal, Popconfirm, Select, Space, Table, Tag, Typography } from 'antd'
import { DeleteOutlined, PlusOutlined, SearchOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AdminPageHead from '../../components/AdminPageHead'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'
import { adminCreateUser, adminListApplicationRoles, adminListApplications, adminListUsers, adminUpdateUserEntitlement, adminUpdateUserStatus, queryKeys, type CreateAdminUserInput } from '../../api/api'
import type { AdminUser, Application } from '../../types'

interface EntitlementFormValue { applicationCode: string; roles: string[] }
interface CreateFormValues { loginName: string; displayName: string; email?: string; password: string; roles: string[]; entitlements?: EntitlementFormValue[] }
interface AccessFormValues { applicationCode: string; enabled: boolean; roles: string[] }

function statusLabel(status: AdminUser['status']) {
  if (status === 'ACTIVE') return <Tag color="success">正常</Tag>
  if (status === 'LOCKED') return <Tag color="warning">已锁定</Tag>
  return <Tag>已停用</Tag>
}

function RoleSelect({ applicationCode, applications }: { applicationCode?: string; applications: Application[] }) {
  const application = applications.find((item) => item.code === applicationCode)
  const rolesQuery = useQuery({ queryKey: queryKeys.applicationRoles(application?.id ?? ''), queryFn: () => adminListApplicationRoles(application!.id), enabled: Boolean(application?.id) })
  return <Select mode="multiple" disabled={!application} loading={rolesQuery.isLoading} placeholder={application ? '选择应用角色；可留空表示仅开通账号' : '请先选择应用'} options={(rolesQuery.data ?? []).filter((role) => role.status === 'ACTIVE').map((role) => ({ value: role.roleKey, label: role.name ? `${role.name}（${role.roleKey}）` : role.roleKey }))} notFoundContent={application && !rolesQuery.isLoading ? '该应用尚未配置角色目录' : undefined} />
}

function CreateEntitlementRow({ fieldName, fieldKey, applications, form, remove }: { fieldName: number; fieldKey: number; applications: Application[]; form: ReturnType<typeof Form.useForm<CreateFormValues>>[0]; remove: (index: number) => void }) {
  const applicationCode = Form.useWatch(['entitlements', fieldName, 'applicationCode'], form)
  return <Space key={fieldKey} align="start" style={{ display: 'flex', width: '100%' }}>
    <Form.Item name={[fieldName, 'applicationCode']} rules={[{ required: true, message: '请选择应用' }]} style={{ width: 220 }}><Select showSearch optionFilterProp="label" placeholder="选择应用" options={applications.map((app) => ({ value: app.code, label: `${app.name}（${app.code}）` }))} /></Form.Item>
    <Form.Item name={[fieldName, 'roles']} style={{ flex: 1, minWidth: 300 }}><RoleSelect applicationCode={applicationCode} applications={applications} /></Form.Item>
    <Button aria-label="移除应用授权" icon={<DeleteOutlined />} onClick={() => remove(fieldName)} />
  </Space>
}

export default function AdminUsers() {
  usePageTitle('账号与访问')
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [accessUser, setAccessUser] = useState<AdminUser | null>(null)
  const [accessApplicationCode, setAccessApplicationCode] = useState('')
  const [createForm] = Form.useForm<CreateFormValues>()
  const [accessForm] = Form.useForm<AccessFormValues>()
  const accessEnabled = Form.useWatch('enabled', accessForm)
  const usersQuery = useQuery({ queryKey: queryKeys.adminUsers, queryFn: adminListUsers })
  const applicationsQuery = useQuery({ queryKey: queryKeys.adminApplications({ userAccess: true }), queryFn: () => adminListApplications({ page: 1, pageSize: 500 }) })
  const applications = useMemo(() => (applicationsQuery.data?.items ?? []).filter((app) => app.status === 'ENABLED'), [applicationsQuery.data?.items])
  const selectedAccessApplication = applications.find((app) => app.code === accessApplicationCode)
  const accessRolesQuery = useQuery({ queryKey: queryKeys.applicationRoles(selectedAccessApplication?.id ?? ''), queryFn: () => adminListApplicationRoles(selectedAccessApplication!.id), enabled: Boolean(selectedAccessApplication?.id && accessUser) })
  const refresh = () => void queryClient.invalidateQueries({ queryKey: queryKeys.adminUsers })

  const createMutation = useMutation({
    mutationFn: (values: CreateFormValues) => {
      const seen = new Set<string>()
      const entitlements = (values.entitlements ?? []).map((item) => {
        if (seen.has(item.applicationCode)) throw new Error('同一个应用只能配置一次初始权限')
        seen.add(item.applicationCode)
        return { applicationCode: item.applicationCode, status: 'ACTIVE' as const, roles: item.roles ?? [] }
      })
      const input: CreateAdminUserInput = { loginName: values.loginName.trim(), displayName: values.displayName.trim(), email: values.email?.trim() || undefined, password: values.password, roles: values.roles, entitlements }
      return adminCreateUser(input)
    },
    onSuccess: () => { message.success('账号已创建，应用权限正在同步'); setCreateOpen(false); createForm.resetFields(); refresh() },
    onError: (error) => message.error(error instanceof Error ? error.message : '账号创建失败'),
  })
  const statusMutation = useMutation({
    mutationFn: ({ user, status }: { user: AdminUser; status: 'ACTIVE' | 'DISABLED' }) => adminUpdateUserStatus(user.id, status),
    onSuccess: (_, variables) => { message.success(variables.status === 'ACTIVE' ? '账号已启用' : '账号已停用，已开通应用的现有会话将被撤销'); refresh() },
    onError: (error) => message.error(error instanceof Error ? error.message : '账号状态更新失败'),
  })
  const accessMutation = useMutation({
    mutationFn: (values: AccessFormValues) => { if (!accessUser || !values.applicationCode) throw new Error('请选择账号和应用'); return adminUpdateUserEntitlement(accessUser.id, values.applicationCode, values.enabled ? 'ACTIVE' : 'DISABLED', values.enabled ? values.roles ?? [] : []) },
    onSuccess: (_, values) => { const app = applications.find((item) => item.code === values.applicationCode); message.success(`${app?.name ?? values.applicationCode}访问权限已更新`); setAccessUser(null); refresh() },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用权限更新失败'),
  })

  const filteredUsers = useMemo(() => {
    const normalized = keyword.trim().toLowerCase()
    if (!normalized) return usersQuery.data ?? []
    return (usersQuery.data ?? []).filter((user) => [user.loginName, user.displayName, user.email].some((value) => value.toLowerCase().includes(normalized)))
  }, [keyword, usersQuery.data])
  const openCreate = () => { createForm.resetFields(); createForm.setFieldsValue({ roles: ['user'], entitlements: [] }); setCreateOpen(true) }
  const loadAccessApplication = (user: AdminUser, applicationCode: string) => { const entitlement = user.entitlements.find((item) => item.applicationCode === applicationCode); setAccessApplicationCode(applicationCode); accessForm.setFieldsValue({ applicationCode, enabled: entitlement?.status === 'ACTIVE', roles: entitlement?.roles ?? [] }) }
  const openAccess = (user: AdminUser) => { const applicationCode = user.entitlements.find((item) => item.status === 'ACTIVE')?.applicationCode ?? applications[0]?.code ?? ''; setAccessUser(user); loadAccessApplication(user, applicationCode) }

  return <div>
    <AdminPageHead title="账号与访问" desc="统一管理登录账号、平台角色和各应用访问权限。密码与多因素认证由统一身份服务安全托管。" extra={<Space><Input allowClear prefix={<SearchOutlined />} placeholder="搜索账号、姓名或邮箱" value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 240 }} /><Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建账号</Button></Space>} />
    {usersQuery.isError || applicationsQuery.isError ? <QueryErrorState refetch={() => { void usersQuery.refetch(); void applicationsQuery.refetch() }} /> : <Table<AdminUser> rowKey="id" loading={usersQuery.isLoading || applicationsQuery.isLoading} dataSource={filteredUsers} scroll={{ x: 1120 }} pagination={{ pageSize: 20, showSizeChanger: false, showTotal: (total) => `共 ${total} 个账号` }} columns={[
      { title: '账号', key: 'account', width: 190, render: (_, user) => <Space direction="vertical" size={0}><Typography.Text strong>{user.displayName || user.loginName}</Typography.Text><Typography.Text type="secondary">{user.loginName}</Typography.Text></Space> },
      { title: '邮箱', dataIndex: 'email', width: 220, render: (value: string) => value || '—' },
      { title: '平台角色', dataIndex: 'roles', width: 180, render: (roles: string[]) => roles.length ? roles.map((role) => <Tag key={role}>{role}</Tag>) : '—' },
      { title: '应用权限', key: 'applications', width: 300, render: (_, user) => { const active = user.entitlements.filter((item) => item.status === 'ACTIVE'); if (!active.length) return <Typography.Text type="secondary">未开通应用</Typography.Text>; return active.map((item) => { const app = applications.find((candidate) => candidate.code === item.applicationCode); return <Tag color="blue" key={item.applicationCode}>{app?.name ?? item.applicationCode}{item.roles.length ? ` · ${item.roles.length} 个角色` : ' · 无角色'}</Tag> }) } },
      { title: '来源', dataIndex: 'identitySource', width: 110, render: (value: string) => value === 'CASDOOR' ? '统一身份' : value },
      { title: '状态', dataIndex: 'status', width: 100, render: statusLabel },
      { title: '操作', key: 'actions', fixed: 'right', width: 190, render: (_, user) => <Space size={4}><Button type="link" size="small" onClick={() => openAccess(user)}>应用权限</Button><Popconfirm title={user.status === 'ACTIVE' ? '停用此账号？' : '启用此账号？'} description={user.status === 'ACTIVE' ? '停用后将撤销统一登录及已开通应用的现有会话。' : '启用后仍需按需开通应用权限。'} okText={user.status === 'ACTIVE' ? '停用' : '启用'} okButtonProps={{ danger: user.status === 'ACTIVE' }} onConfirm={() => statusMutation.mutate({ user, status: user.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE' })}><Button type="link" size="small" danger={user.status === 'ACTIVE'}>{user.status === 'ACTIVE' ? '停用' : '启用'}</Button></Popconfirm></Space> },
    ]} />}

    <Modal title="新建账号" open={createOpen} width={760} destroyOnHidden confirmLoading={createMutation.isPending} okText="创建账号" cancelText="取消" onCancel={() => setCreateOpen(false)} onOk={() => createForm.submit()}>
      <Form form={createForm} layout="vertical" requiredMark={false} style={{ marginTop: 20 }} onFinish={(values) => createMutation.mutate(values)}>
        <div className="velora-form-grid"><Form.Item name="loginName" label="登录账号" rules={[{ required: true, message: '请输入登录账号' }, { pattern: /^[a-zA-Z][a-zA-Z0-9._-]{2,63}$/, message: '3–64 位，以字母开头，可包含数字、点、下划线和短横线' }]}><Input autoComplete="off" placeholder="如 carson" /></Form.Item><Form.Item name="displayName" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}><Input placeholder="用于门户和应用展示" /></Form.Item></div>
        <Form.Item name="email" label="邮箱" rules={[{ type: 'email', message: '请输入有效的邮箱地址' }]}><Input autoComplete="off" placeholder="选填" /></Form.Item>
        <Form.Item name="password" label="初始密码" extra="密码不会保存在 Velora；账号创建后由统一身份服务托管。" rules={[{ required: true, message: '请输入初始密码' }, { min: 12, message: '密码至少 12 位' }]}><Input.Password autoComplete="new-password" placeholder="至少 12 位，包含大小写字母、数字和特殊字符" /></Form.Item>
        <Form.Item name="roles" label="平台角色" rules={[{ required: true, message: '请选择平台角色' }]}><Select mode="multiple" options={[{ label: '普通用户', value: 'user' }, { label: '审计员', value: 'auditor' }]} /></Form.Item>
        <Typography.Title level={5}>初始应用权限</Typography.Title><Typography.Paragraph type="secondary">可选。未开通应用是合法状态，账号创建后也可以继续配置。</Typography.Paragraph>
        <Form.List name="entitlements">{(fields, { add, remove }) => <Space direction="vertical" style={{ width: '100%' }}>{fields.map((field) => <CreateEntitlementRow key={field.key} fieldName={field.name} fieldKey={field.key} applications={applications} form={createForm} remove={remove} />)}<Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ roles: [] })} disabled={!applications.length}>添加应用权限</Button></Space>}</Form.List>
      </Form>
    </Modal>

    <Modal title={accessUser ? `应用权限 · ${accessUser.displayName || accessUser.loginName}` : '应用权限'} open={Boolean(accessUser)} width={620} destroyOnHidden okText="保存" cancelText="取消" confirmLoading={accessMutation.isPending} onCancel={() => setAccessUser(null)} onOk={() => accessForm.submit()}>
      <Form form={accessForm} layout="vertical" requiredMark={false} style={{ marginTop: 20 }} onFinish={(values) => accessMutation.mutate(values)}>
        <Form.Item name="applicationCode" label="应用" rules={[{ required: true, message: '请选择应用' }]}><Select showSearch optionFilterProp="label" options={applications.map((app) => ({ value: app.code, label: `${app.name}（${app.code}）` }))} onChange={(value) => accessUser && loadAccessApplication(accessUser, value)} /></Form.Item>
        <Form.Item name="enabled" valuePropName="checked"><Checkbox>允许访问此应用</Checkbox></Form.Item>
        {accessEnabled ? <Form.Item name="roles" label="应用角色"><Select mode="multiple" loading={accessRolesQuery.isLoading} placeholder="可留空，表示账号已开通但暂无业务角色" options={(accessRolesQuery.data ?? []).filter((role) => role.status === 'ACTIVE').map((role) => ({ value: role.roleKey, label: role.name ? `${role.name}（${role.roleKey}）` : role.roleKey }))} /></Form.Item> : <Typography.Text type="secondary">关闭后会下发停用事件，由目标应用撤销现有会话和访问令牌。</Typography.Text>}
      </Form>
    </Modal>
  </div>
}
