import { useMemo, useState } from 'react'
import {
  App as AntdApp,
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import { PlusOutlined, SearchOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import AdminPageHead from '../../components/AdminPageHead'
import QueryErrorState from '../../components/QueryErrorState'
import { usePageTitle } from '../../hooks/usePageTitle'
import {
  adminCreateUser,
  adminListUsers,
  adminUpdateUserEntitlement,
  adminUpdateUserStatus,
  queryKeys,
  type CreateAdminUserInput,
} from '../../api/api'
import type { AdminUser } from '../../types'

const SPECTRA_ROLES = [
  { label: '开发者', value: 'developer' },
  { label: '审计员', value: 'auditor' },
  { label: '项目管理员', value: 'project_admin' },
  { label: '安全管理员', value: 'security_admin' },
  { label: '系统管理员', value: 'system_admin' },
  { label: 'CI 服务账号', value: 'ci_service' },
]

interface CreateFormValues {
  loginName: string
  displayName: string
  email?: string
  password: string
  roles: string[]
  spectraEnabled?: boolean
  spectraRoles?: string[]
}

interface AccessFormValues {
  enabled: boolean
  roles: string[]
}

function statusLabel(status: AdminUser['status']) {
  if (status === 'ACTIVE') return <Tag color="success">正常</Tag>
  if (status === 'LOCKED') return <Tag color="warning">已锁定</Tag>
  return <Tag>已停用</Tag>
}

export default function AdminUsers() {
  usePageTitle('账号与访问')
  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [accessUser, setAccessUser] = useState<AdminUser | null>(null)
  const [createForm] = Form.useForm<CreateFormValues>()
  const [accessForm] = Form.useForm<AccessFormValues>()
  const spectraEnabled = Form.useWatch('spectraEnabled', createForm)
  const accessEnabled = Form.useWatch('enabled', accessForm)

  const usersQuery = useQuery({ queryKey: queryKeys.adminUsers, queryFn: adminListUsers })
  const refresh = () => void queryClient.invalidateQueries({ queryKey: queryKeys.adminUsers })

  const createMutation = useMutation({
    mutationFn: (values: CreateFormValues) => {
      const input: CreateAdminUserInput = {
        loginName: values.loginName.trim(),
        displayName: values.displayName.trim(),
        email: values.email?.trim() || undefined,
        password: values.password,
        roles: values.roles,
        entitlements: values.spectraEnabled
          ? [{ applicationCode: 'spectra', status: 'ACTIVE', roles: values.spectraRoles ?? [] }]
          : [],
      }
      return adminCreateUser(input)
    },
    onSuccess: () => {
      message.success('账号已创建，应用权限正在同步')
      setCreateOpen(false)
      createForm.resetFields()
      refresh()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '账号创建失败'),
  })

  const statusMutation = useMutation({
    mutationFn: ({ user, status }: { user: AdminUser; status: 'ACTIVE' | 'DISABLED' }) =>
      adminUpdateUserStatus(user.id, status),
    onSuccess: (_, variables) => {
      message.success(variables.status === 'ACTIVE' ? '账号已启用' : '账号已停用，现有会话已撤销')
      refresh()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '账号状态更新失败'),
  })

  const accessMutation = useMutation({
    mutationFn: (values: AccessFormValues) => {
      if (!accessUser) throw new Error('未选择账号')
      return adminUpdateUserEntitlement(
        accessUser.id,
        'spectra',
        values.enabled ? 'ACTIVE' : 'DISABLED',
        values.enabled ? values.roles : [],
      )
    },
    onSuccess: () => {
      message.success('Spectra 访问权限已更新')
      setAccessUser(null)
      refresh()
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用权限更新失败'),
  })

  const filteredUsers = useMemo(() => {
    const normalized = keyword.trim().toLowerCase()
    if (!normalized) return usersQuery.data ?? []
    return (usersQuery.data ?? []).filter((user) =>
      [user.loginName, user.displayName, user.email].some((value) => value.toLowerCase().includes(normalized)),
    )
  }, [keyword, usersQuery.data])

  const openCreate = () => {
    createForm.resetFields()
    createForm.setFieldsValue({ roles: ['user'], spectraEnabled: true, spectraRoles: ['developer'] })
    setCreateOpen(true)
  }

  const openAccess = (user: AdminUser) => {
    const entitlement = user.entitlements.find((item) => item.applicationCode === 'spectra')
    accessForm.setFieldsValue({ enabled: entitlement?.status === 'ACTIVE', roles: entitlement?.roles ?? ['developer'] })
    setAccessUser(user)
  }

  return (
    <div>
      <AdminPageHead
        title="账号与访问"
        desc="统一管理登录账号、平台角色和应用访问权限。密码与多因素认证由统一身份服务安全托管。"
        extra={
          <Space>
            <Input
              allowClear
              prefix={<SearchOutlined />}
              placeholder="搜索账号、姓名或邮箱"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              style={{ width: 240 }}
            />
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建账号
            </Button>
          </Space>
        }
      />

      {usersQuery.isError ? (
        <QueryErrorState refetch={usersQuery.refetch} />
      ) : (
        <Table<AdminUser>
          rowKey="id"
          loading={usersQuery.isLoading}
          dataSource={filteredUsers}
          scroll={{ x: 1080 }}
          pagination={{ pageSize: 20, showSizeChanger: false, showTotal: (total) => `共 ${total} 个账号` }}
          columns={[
            {
              title: '账号',
              key: 'account',
              width: 190,
              render: (_, user) => (
                <Space direction="vertical" size={0}>
                  <Typography.Text strong>{user.displayName || user.loginName}</Typography.Text>
                  <Typography.Text type="secondary">{user.loginName}</Typography.Text>
                </Space>
              ),
            },
            { title: '邮箱', dataIndex: 'email', width: 220, render: (value: string) => value || '—' },
            {
              title: '平台角色',
              dataIndex: 'roles',
              width: 180,
              render: (roles: string[]) => roles.length ? roles.map((role) => <Tag key={role}>{role}</Tag>) : '—',
            },
            {
              title: 'Spectra 权限',
              key: 'spectra',
              width: 230,
              render: (_, user) => {
                const entitlement = user.entitlements.find((item) => item.applicationCode === 'spectra')
                if (!entitlement || entitlement.status !== 'ACTIVE') return <Typography.Text type="secondary">未开通</Typography.Text>
                return entitlement.roles.map((role) => <Tag color="blue" key={role}>{role}</Tag>)
              },
            },
            { title: '来源', dataIndex: 'identitySource', width: 110, render: (value: string) => value === 'CASDOOR' ? '统一身份' : value },
            { title: '状态', dataIndex: 'status', width: 100, render: statusLabel },
            {
              title: '操作',
              key: 'actions',
              fixed: 'right',
              width: 190,
              render: (_, user) => (
                <Space size={4}>
                  <Button type="link" size="small" onClick={() => openAccess(user)}>应用权限</Button>
                  <Popconfirm
                    title={user.status === 'ACTIVE' ? '停用此账号？' : '启用此账号？'}
                    description={user.status === 'ACTIVE' ? '停用后将撤销统一登录及 Spectra 现有会话。' : '启用后仍需按需开通应用权限。'}
                    okText={user.status === 'ACTIVE' ? '停用' : '启用'}
                    okButtonProps={{ danger: user.status === 'ACTIVE' }}
                    onConfirm={() => statusMutation.mutate({ user, status: user.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE' })}
                  >
                    <Button type="link" size="small" danger={user.status === 'ACTIVE'}>
                      {user.status === 'ACTIVE' ? '停用' : '启用'}
                    </Button>
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
        />
      )}

      <Modal
        title="新建账号"
        open={createOpen}
        width={680}
        destroyOnHidden
        confirmLoading={createMutation.isPending}
        okText="创建账号"
        cancelText="取消"
        onCancel={() => setCreateOpen(false)}
        onOk={() => createForm.submit()}
      >
        <Form form={createForm} layout="vertical" requiredMark={false} style={{ marginTop: 20 }} onFinish={(values) => createMutation.mutate(values)}>
          <div className="velora-form-grid">
            <Form.Item name="loginName" label="登录账号" rules={[{ required: true, message: '请输入登录账号' }, { pattern: /^[a-zA-Z][a-zA-Z0-9._-]{2,63}$/, message: '3–64 位，以字母开头，可包含数字、点、下划线和短横线' }]}>
              <Input autoComplete="off" placeholder="如 carson" />
            </Form.Item>
            <Form.Item name="displayName" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
              <Input placeholder="用于门户和应用展示" />
            </Form.Item>
          </div>
          <Form.Item name="email" label="邮箱" rules={[{ type: 'email', message: '请输入有效的邮箱地址' }]}>
            <Input autoComplete="off" placeholder="选填" />
          </Form.Item>
          <Form.Item name="password" label="初始密码" extra="密码不会保存在 Velora；账号创建后由统一身份服务托管。" rules={[{ required: true, message: '请输入初始密码' }, { min: 12, message: '密码至少 12 位' }]}>
            <Input.Password autoComplete="new-password" placeholder="至少 12 位，包含大小写字母、数字和特殊字符" />
          </Form.Item>
          <Form.Item name="roles" label="平台角色" rules={[{ required: true, message: '请选择平台角色' }]}>
            <Select mode="multiple" options={[{ label: '普通用户', value: 'user' }, { label: '审计员', value: 'auditor' }]} />
          </Form.Item>
          <Form.Item name="spectraEnabled" valuePropName="checked">
            <Checkbox>同时开通 Spectra</Checkbox>
          </Form.Item>
          {spectraEnabled ? (
            <Form.Item name="spectraRoles" label="Spectra 角色" rules={[{ required: true, message: '请选择至少一个 Spectra 角色' }]}>
              <Select mode="multiple" options={SPECTRA_ROLES} placeholder="选择应用内权限" />
            </Form.Item>
          ) : null}
        </Form>
      </Modal>

      <Modal
        title={accessUser ? `Spectra 权限 · ${accessUser.displayName || accessUser.loginName}` : 'Spectra 权限'}
        open={Boolean(accessUser)}
        width={560}
        destroyOnHidden
        okText="保存"
        cancelText="取消"
        confirmLoading={accessMutation.isPending}
        onCancel={() => setAccessUser(null)}
        onOk={() => accessForm.submit()}
      >
        <Form form={accessForm} layout="vertical" requiredMark={false} style={{ marginTop: 20 }} onFinish={(values) => accessMutation.mutate(values)}>
          <Form.Item name="enabled" valuePropName="checked">
            <Checkbox>允许访问 Spectra</Checkbox>
          </Form.Item>
          {accessEnabled ? (
            <Form.Item name="roles" label="应用角色" rules={[{ required: true, message: '请选择至少一个应用角色' }]}>
              <Select mode="multiple" options={SPECTRA_ROLES} />
            </Form.Item>
          ) : (
            <Typography.Text type="secondary">关闭后会撤销该账号在 Spectra 的现有会话和访问令牌。</Typography.Text>
          )}
        </Form>
      </Modal>
    </div>
  )
}
