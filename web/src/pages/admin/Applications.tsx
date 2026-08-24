import { useRef, useState } from 'react'
import { App, Button, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProFormSelect, ProFormText, ProFormTextArea, ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { adminCreateApplication, adminListApplications, type AdminApplicationInput } from '../../api/api'
import { AppIcon } from '../../components/AppCard'
import { APP_LIFECYCLE_COLOR, APP_LIFECYCLE_LABEL, SSO_TYPE_LABEL, enumLabel } from '../../labels'
import type { Application } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { PORTAL_MANAGE } from '../../auth/permissions'
import { useAdminPermission } from '../../auth/useAdminPermission'
import QueryErrorState from '../../components/QueryErrorState'

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
  const create = useMutation({
    mutationFn: (values: CreateApplication) => adminCreateApplication({ ...values, status: 'ENABLED', sort: 0, isFeatured: false }),
    onSuccess: async (application) => { message.success('应用已创建'); setOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] }); navigate(`/admin/applications/${application.id}`) },
    onError: (error) => message.error(error instanceof Error ? error.message : '应用创建失败'),
  })
  const columns: ProColumns<Application>[] = [
    { title: '应用', dataIndex: 'keyword', render: (_, row) => <Space><AppIcon app={row} size={36} /><Space direction="vertical" size={0}><Link to={`/admin/applications/${row.id}`}><Typography.Text strong>{row.name}</Typography.Text></Link><Typography.Text type="secondary">{row.code}</Typography.Text></Space></Space> },
    { title: '登录方式', dataIndex: 'ssoType', valueType: 'select', valueEnum: { URL: { text: '普通链接' }, OIDC: { text: '统一登录' } }, render: (_, row) => SSO_TYPE_LABEL[row.ssoType] },
    { title: '接入状态', dataIndex: 'lifecycleStatus', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(APP_LIFECYCLE_LABEL).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag color={APP_LIFECYCLE_COLOR[row.lifecycleStatus ?? '']}>{enumLabel(APP_LIFECYCLE_LABEL, row.lifecycleStatus, '草稿')}</Tag> },
    { title: '使用状态', dataIndex: 'status', valueType: 'select', valueEnum: { ENABLED: { text: '可用' }, DISABLED: { text: '已停用' } }, render: (_, row) => <Tag color={row.status === 'ENABLED' ? 'success' : 'default'}>{row.status === 'ENABLED' ? '可用' : '已停用'}</Tag> },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => <Link to={`/admin/applications/${row.id}`}>{canCreate ? '管理' : '查看'}</Link> },
  ]
  return <PageContainer title="应用管理">
    {loadError ? <QueryErrorState refetch={() => setLoadError(false)} /> : <ProTable<Application> className="velora-admin-primary-table" actionRef={actionRef} rowKey="id" columns={columns} search={{ labelWidth: 'auto' }} request={async (params) => { try { const data = await adminListApplications({ page: params.current, pageSize: params.pageSize, keyword: String(params.keyword ?? '') || undefined, status: String(params.status ?? '') || undefined, ssoType: String(params.ssoType ?? '') || undefined, lifecycleStatus: String(params.lifecycleStatus ?? '') || undefined }); return { data: data.items, total: data.total, success: true } } catch { setLoadError(true); return { data: [], total: 0, success: false } } }} pagination={{ defaultPageSize: 20, showSizeChanger: true }} toolBarRender={canCreate ? () => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => setOpen(true)}>新建应用</Button>] : false} />}
    <ModalForm<CreateApplication> title="新建应用" open={open} onOpenChange={setOpen} width={600} initialValues={{ ssoType: 'OIDC' }} submitter={{ searchConfig: { submitText: '创建应用', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
      <ProFormText name="name" label="应用名称" rules={[{ required: true, message: '请输入应用名称' }]} />
      <ProFormText name="code" label="应用编码" rules={[{ required: true, message: '请输入应用编码' }, { pattern: /^[a-z][a-z0-9_-]{1,63}$/, message: '使用小写字母、数字、短横线或下划线' }]} />
      <ProFormSelect name="ssoType" label="登录方式" options={[{ label: '统一登录', value: 'OIDC' }, { label: '普通链接', value: 'URL' }]} rules={[{ required: true }]} />
      <ProFormText name="homeUrl" label="应用地址" rules={[{ required: true, message: '请输入应用地址' }, { type: 'url', message: '请输入完整 HTTPS 地址' }, { pattern: /^https:\/\//i, message: '生产环境只允许 HTTPS' }]} />
      <ProFormTextArea name="description" label="用途" fieldProps={{ maxLength: 200, showCount: true }} />
    </ModalForm>
  </PageContainer>
}
