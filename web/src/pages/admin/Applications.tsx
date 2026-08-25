import { useRef, useState } from 'react'
import { App, Button, Popconfirm, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProFormSelect, ProFormText, ProFormTextArea, ProList, type ActionType, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { adminCreateApplication, adminListApplications, adminUpdateApplication, type AdminApplicationInput } from '../../api/api'
import { AppIcon } from '../../components/AppCard'
import { APP_LIFECYCLE_COLOR, APP_LIFECYCLE_LABEL, SSO_TYPE_LABEL, enumLabel } from '../../labels'
import type { Application } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { PORTAL_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import QueryErrorState from '../../components/QueryErrorState'
import { AdminListScope, AdminListSearch } from '../../components/admin/AdminListToolbar'

type CreateApplication = Pick<AdminApplicationInput, 'code' | 'name' | 'description' | 'homeUrl' | 'ssoType'>

export default function Applications() {
  usePageTitle('应用')
  const { message } = App.useApp()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const actionRef = useRef<ActionType>(null)
  const canCreate = useAdminPermission(PORTAL_MANAGE)
  const [open, setOpen] = useState(false)
  const [loadError, setLoadError] = useState(false)
  const [scope, setScope] = useState('ALL')
  const [searchValue, setSearchValue] = useState('')
  const [keyword, setKeyword] = useState('')
  const counts = useQuery({
    queryKey: ['admin', 'application-counts'],
    queryFn: async () => {
      const [all, enabled, disabled] = await Promise.all([
        adminListApplications({ page: 1, pageSize: 1 }),
        adminListApplications({ page: 1, pageSize: 1, status: 'ENABLED' }),
        adminListApplications({ page: 1, pageSize: 1, status: 'DISABLED' }),
      ])
      return { ALL: all.total, ENABLED: enabled.total, DISABLED: disabled.total }
    },
  })
  const create = useMutation({
    mutationFn: (values: CreateApplication) => adminCreateApplication({ ...values, status: 'ENABLED', sort: 0, isFeatured: false }),
    onSuccess: async (application) => { message.success('应用已创建'); setOpen(false); await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'application-counts'] })]); navigate(`/admin/applications/${application.id}`) },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用创建失败'),
  })
  const updateStatus = useMutation({
    mutationFn: (application: Application) => adminUpdateApplication(application.id, {
      code: application.code,
      name: application.name,
      description: application.description,
      icon: application.icon,
      categoryId: application.categoryId,
      homeUrl: application.homeUrl,
      launchUrl: application.launchUrl,
      ssoType: application.ssoType,
      ownerUserId: application.ownerUserId,
      ownerDepartmentId: application.ownerDepartmentId,
      status: application.status === 'ENABLED' ? 'DISABLED' : 'ENABLED',
      sort: application.sort,
      isFeatured: application.isFeatured,
      tagIds: application.tags.map((item) => item.id),
    }),
    onSuccess: async (application) => { message.success(application.status === 'ENABLED' ? '应用已启用' : '应用已停用'); await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'application-counts'] })]); actionRef.current?.reload() },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用状态更新失败'),
  })
  const columns: ProColumns<Application>[] = [
    { dataIndex: 'icon', listSlot: 'avatar', search: false, render: (_, row) => <AppIcon app={row} size={40} /> },
    { title: '应用', dataIndex: 'keyword', listSlot: 'title', render: (_, row) => <Link to={`/admin/applications/${row.id}`}><Typography.Text strong>{row.name}</Typography.Text></Link> },
    { dataIndex: 'code', listSlot: 'description', search: false, render: (_, row) => <Typography.Text type="secondary">{row.code}{row.description ? ` · ${row.description}` : ''}</Typography.Text> },
    { title: '使用状态', dataIndex: 'status', listSlot: 'subTitle', valueType: 'select', valueEnum: { ENABLED: { text: '可用' }, DISABLED: { text: '已停用' } }, render: (_, row) => <Space size={4}><Tag color={row.status === 'ENABLED' ? 'success' : 'default'}>{row.status === 'ENABLED' ? '可用' : '已停用'}</Tag><Tag color={APP_LIFECYCLE_COLOR[row.lifecycleStatus ?? '']}>{enumLabel(APP_LIFECYCLE_LABEL, row.lifecycleStatus, '草稿')}</Tag><Tag>{SSO_TYPE_LABEL[row.ssoType]}</Tag></Space> },
    { title: '登录方式', dataIndex: 'ssoType', valueType: 'select', valueEnum: { URL: { text: '普通链接' }, OIDC: { text: '统一登录' } } },
    { title: '接入状态', dataIndex: 'lifecycleStatus', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(APP_LIFECYCLE_LABEL).map(([key, text]) => [key, { text }])) },
    { title: '操作', listSlot: 'actions', valueType: 'option', search: false, render: (_, row) => [<Link key="manage" to={`/admin/applications/${row.id}`}>{canCreate ? '管理' : '查看'}</Link>, canCreate ? <Popconfirm key="status" title={row.status === 'ENABLED' ? '停用此应用？' : '启用此应用？'} description={row.status === 'ENABLED' ? '停用后，用户将无法从门户进入该应用。' : undefined} okText={row.status === 'ENABLED' ? '停用' : '启用'} okButtonProps={{ danger: row.status === 'ENABLED' }} onConfirm={() => updateStatus.mutate(row)}><Button type="link" danger={row.status === 'ENABLED'} loading={updateStatus.isPending}>{row.status === 'ENABLED' ? '停用' : '启用'}</Button></Popconfirm> : null].filter(Boolean) },
  ]
  return <PageContainer title="应用管理">
    {loadError ? <QueryErrorState refetch={() => setLoadError(false)} /> : <ProList<Application>
      key={`${scope}:${keyword}`}
      className="velora-admin-primary-table velora-admin-entity-list"
      actionRef={actionRef}
      rowKey="id"
      columns={columns}
      search={false}
      headerTitle={<AdminListScope value={scope} onChange={setScope} options={[
        { label: '全部应用', value: 'ALL', count: counts.data?.ALL },
        { label: '可用', value: 'ENABLED', count: counts.data?.ENABLED },
        { label: '已停用', value: 'DISABLED', count: counts.data?.DISABLED },
      ]} />}
      request={async (params) => { try { const data = await adminListApplications({ page: params.current, pageSize: params.pageSize, keyword: keyword || undefined, status: scope === 'ALL' ? undefined : scope }); return { data: data.items, total: data.total, success: true } } catch { setLoadError(true); return { data: [], total: 0, success: false } } }}
      pagination={{ defaultPageSize: 20, showSizeChanger: true }}
      toolBarRender={() => [<AdminListSearch key="search" value={searchValue} placeholder="搜索应用名称或编码" onChange={setSearchValue} onSearch={setKeyword}>{canCreate && <Button type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>新建应用</Button>}</AdminListSearch>]}
    />}
    <ModalForm<CreateApplication> title="新建应用" open={open} onOpenChange={setOpen} width={600} initialValues={{ ssoType: 'OIDC' }} submitter={{ searchConfig: { submitText: '创建应用', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
      <ProFormText name="name" label="应用名称" rules={[{ required: true, message: '请输入应用名称' }]} />
      <ProFormText name="code" label="应用编码" rules={[{ required: true, message: '请输入应用编码' }, { pattern: /^[a-z][a-z0-9_-]{1,63}$/, message: '使用小写字母、数字、短横线或下划线' }]} />
      <ProFormSelect name="ssoType" label="登录方式" options={[{ label: '统一登录', value: 'OIDC' }, { label: '普通链接', value: 'URL' }]} rules={[{ required: true }]} />
      <ProFormText name="homeUrl" label="应用地址" rules={[{ required: true, message: '请输入应用地址' }, { type: 'url', message: '请输入完整 HTTPS 地址' }, { pattern: /^https:\/\//i, message: '生产环境只允许 HTTPS' }]} />
      <ProFormTextArea name="description" label="用途" fieldProps={{ maxLength: 200, showCount: true }} />
    </ModalForm>
  </PageContainer>
}
