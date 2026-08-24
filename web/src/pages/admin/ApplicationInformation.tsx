import { useState } from 'react'
import { App, Button, Tag } from 'antd'
import { DrawerForm, ProDescriptions, ProForm, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminUpdateApplication, listCategories, listTags, queryKeys, type AdminApplicationInput } from '../../api/api'
import { listDepartments } from '../../api/admin-platform'
import type { Application } from '../../types'
import { SSO_TYPE_LABEL } from '../../labels'
import AdminUserSelect from '../../components/AdminUserSelect'
import QueryErrorState from '../../components/QueryErrorState'

interface Props { application: Application }

export default function ApplicationInformation({ application }: Props) {
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const departments = useQuery({ queryKey: ['admin', 'departments'], queryFn: listDepartments })
  const categories = useQuery({ queryKey: ['categories'], queryFn: listCategories })
  const tags = useQuery({ queryKey: ['tags'], queryFn: listTags })
  const editOptionsError = departments.isError || categories.isError || tags.isError
  const initialValues: AdminApplicationInput = { code: application.code, name: application.name, description: application.description, icon: application.icon, categoryId: application.categoryId, homeUrl: application.homeUrl, launchUrl: application.launchUrl, ssoType: application.ssoType, ownerUserId: application.ownerUserId, ownerDepartmentId: application.ownerDepartmentId, status: application.status, sort: application.sort, isFeatured: application.isFeatured, tagIds: application.tags.map((item) => item.id) }
  const update = useMutation({ mutationFn: (values: AdminApplicationInput) => adminUpdateApplication(application.id, { ...initialValues, ...values, code: application.code }), onSuccess: async () => { message.success('应用信息已更新'); setOpen(false); await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'applications'] }), queryClient.invalidateQueries({ queryKey: queryKeys.applicationOnboarding(application.id) })]) }, onError: (error) => message.error(error instanceof Error ? error.message : '应用信息保存失败') })
  return <>
    <ProDescriptions<Application> className="velora-admin-section-card" dataSource={application} column={2} extra={<Button type="primary" onClick={() => setOpen(true)}>编辑信息</Button>} columns={[
      { title: '应用名称', dataIndex: 'name' },
      { title: '应用编码', dataIndex: 'code' },
      { title: '负责人', dataIndex: 'owner', render: (_, row) => row.owner || '未设置' },
      { title: '所属部门', dataIndex: 'department', render: (_, row) => row.department || '未设置' },
      { title: '登录方式', dataIndex: 'ssoType', render: (_, row) => SSO_TYPE_LABEL[row.ssoType] },
      { title: '使用状态', dataIndex: 'status', render: (_, row) => <Tag color={row.status === 'ENABLED' ? 'success' : 'default'}>{row.status === 'ENABLED' ? '可用' : '已停用'}</Tag> },
      { title: '应用主页', dataIndex: 'homeUrl' },
      { title: '打开应用地址', dataIndex: 'launchUrl', render: (_, row) => row.launchUrl || row.homeUrl || '—' },
      { title: '分类', render: (_, row) => row.category?.name || '未分类' },
      { title: '用途', dataIndex: 'description', span: 2, render: (_, row) => row.description || '—' },
    ]} />
    <DrawerForm<AdminApplicationInput> key={`${application.id}-${application.configVersion}`} title="编辑应用信息" open={open} onOpenChange={setOpen} width={620} initialValues={initialValues} submitter={editOptionsError ? false : { searchConfig: { submitText: '保存', resetText: '取消' } }} onFinish={async (values) => { await update.mutateAsync(values); return true }}>
      {editOptionsError ? <QueryErrorState compact description="编辑选项加载失败，请重试。" refetch={() => Promise.all([departments.refetch(), categories.refetch(), tags.refetch()])} /> : <>
      <ProFormText name="name" label="应用名称" rules={[{ required: true, message: '请输入应用名称' }]} />
      <ProFormTextArea name="description" label="用途" fieldProps={{ maxLength: 200, showCount: true }} />
      <ProForm.Item name="ownerUserId" label="负责人"><AdminUserSelect /></ProForm.Item>
      <ProFormSelect name="ownerDepartmentId" label="所属部门" showSearch fieldProps={{ optionFilterProp: 'label' }} options={(departments.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.name, value: item.id }))} />
      <ProFormSelect name="categoryId" label="分类" options={(categories.data ?? []).map((item) => ({ label: item.name, value: item.id }))} />
      <ProFormSelect name="tagIds" label="标签" mode="multiple" options={(tags.data ?? []).map((item) => ({ label: item.name, value: item.id }))} />
      <ProFormText name="icon" label="图标地址" />
      <ProFormText name="homeUrl" label="应用主页" rules={[{ required: true }, { type: 'url' }, { pattern: /^https:\/\//i, message: '生产环境只允许 HTTPS' }]} />
      <ProFormText name="launchUrl" label="打开应用地址" rules={[{ type: 'url' }, { pattern: /^https:\/\//i, message: '生产环境只允许 HTTPS' }]} />
      <ProFormSelect name="status" label="使用状态" options={[{ label: '可用', value: 'ENABLED' }, { label: '停用', value: 'DISABLED' }]} rules={[{ required: true }]} />
      </>}
    </DrawerForm>
  </>
}
