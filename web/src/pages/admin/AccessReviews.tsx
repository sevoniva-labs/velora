import { useState } from 'react'
import { App, Button, Drawer, Form, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { ModalForm, PageContainer, ProFormDateTimePicker, ProFormRadio, ProFormSelect, ProFormTextArea, ProTable, type ProColumns } from '@ant-design/pro-components'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs, { type Dayjs } from 'dayjs'
import { adminListUsers } from '../../api/api'
import { createAccessReview, decideAccessReviewItem, listAccessReviewItems, listAccessReviews, listPlatformRoles } from '../../api/admin-platform'
import type { AccessReview, AccessReviewItem } from '../../types'
import { usePageTitle } from '../../hooks/usePageTitle'
import { formatDateTime } from '../../utils/format'

interface CreateForm { reviewerId: string; dueAt: Dayjs }
interface DecisionForm { decision: 'APPROVE' | 'REVOKE'; reason: string }
const REVIEW_LABELS: Record<string, string> = { OPEN: '进行中', COMPLETED: '已完成', EXPIRED: '已过期' }

export default function AccessReviews() {
  usePageTitle('访问复核')
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [selected, setSelected] = useState<AccessReview>()
  const [deciding, setDeciding] = useState<AccessReviewItem>()
  const [decisionForm] = Form.useForm<DecisionForm>()
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: adminListUsers })
  const reviews = useQuery({ queryKey: ['admin', 'access-reviews'], queryFn: listAccessReviews })
  const roles = useQuery({ queryKey: ['admin', 'roles'], queryFn: listPlatformRoles })
  const items = useQuery({ queryKey: ['admin', 'access-reviews', selected?.id, 'items'], queryFn: () => listAccessReviewItems(selected!.id), enabled: Boolean(selected) })
  const create = useMutation({ mutationFn: (values: CreateForm) => createAccessReview(values.reviewerId, values.dueAt.toISOString()), onSuccess: async () => { message.success('访问复核已发起'); setCreateOpen(false); await queryClient.invalidateQueries({ queryKey: ['admin', 'access-reviews'] }) }, onError: (error) => message.error(error instanceof Error ? error.message : '访问复核创建失败') })
  const decide = useMutation({ mutationFn: (values: DecisionForm) => decideAccessReviewItem(selected!.id, deciding!.id, values.decision, values.reason ?? ''), onSuccess: async () => { message.success('复核结果已提交'); setDeciding(undefined); await Promise.all([queryClient.invalidateQueries({ queryKey: ['admin', 'access-reviews', selected?.id, 'items'] }), queryClient.invalidateQueries({ queryKey: ['admin', 'access-reviews'] })]) }, onError: (error) => message.error(error instanceof Error ? error.message : '复核处理失败') })
  const columns: ProColumns<AccessReview>[] = [
    { title: '复核负责人', dataIndex: 'reviewerName' },
    { title: '截止时间', dataIndex: 'dueAt', valueType: 'dateTime', render: (_, row) => formatDateTime(row.dueAt) },
    { title: '发起时间', dataIndex: 'createdAt', valueType: 'dateTime', search: false, render: (_, row) => formatDateTime(row.createdAt) },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(REVIEW_LABELS).map(([key, text]) => [key, { text }])), render: (_, row) => <Tag color={row.status === 'OPEN' ? 'processing' : row.status === 'COMPLETED' ? 'success' : 'default'}>{REVIEW_LABELS[row.status] ?? '已结束'}</Tag> },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => <Button type="link" onClick={() => setSelected(row)}>查看</Button> },
  ]
  const itemColumns: ProColumns<AccessReviewItem>[] = [
    { title: '用户', dataIndex: 'loginName' },
    { title: '平台角色', dataIndex: 'roleKey', render: (_, row) => roles.data?.find((role) => role.key === row.roleKey)?.name ?? row.roleKey },
    { title: '复核结果', dataIndex: 'decision', render: (_, row) => row.decision ? <Tag color={row.decision === 'APPROVE' ? 'success' : 'warning'}>{row.decision === 'APPROVE' ? '保留' : row.decision === 'REVOKE' ? '撤销' : '例外'}</Tag> : <Tag>待复核</Tag> },
    { title: '原因', dataIndex: 'reason', ellipsis: true },
    { title: '操作', valueType: 'option', width: 90, render: (_, row) => row.decision ? <Typography.Text type="secondary">已处理</Typography.Text> : <Button type="link" onClick={() => setDeciding(row)}>复核</Button> },
  ]
  return <PageContainer title="访问复核"><ProTable<AccessReview> rowKey="id" columns={columns} dataSource={reviews.data ?? []} loading={reviews.isLoading} search={{ labelWidth: 'auto' }} pagination={{ pageSize: 20 }} toolBarRender={() => [<Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>发起复核</Button>]} />
    <ModalForm<CreateForm> title="发起访问复核" open={createOpen} onOpenChange={setCreateOpen} initialValues={{ dueAt: dayjs().add(7, 'day') }} submitter={{ searchConfig: { submitText: '发起复核', resetText: '取消' } }} onFinish={async (values) => { await create.mutateAsync(values); return true }}>
      <ProFormSelect name="reviewerId" label="复核负责人" showSearch fieldProps={{ optionFilterProp: 'label' }} options={(users.data ?? []).filter((item) => item.status === 'ACTIVE').map((item) => ({ label: item.displayName || item.loginName, value: item.id }))} rules={[{ required: true, message: '请选择复核负责人' }]} />
      <ProFormDateTimePicker name="dueAt" label="截止时间" rules={[{ required: true, message: '请选择截止时间' }]} />
    </ModalForm>
    <Drawer title={selected ? `访问复核 · ${selected.reviewerName}` : '访问复核'} open={Boolean(selected)} onClose={() => setSelected(undefined)} width={760}><ProTable<AccessReviewItem> rowKey="id" columns={itemColumns} dataSource={items.data ?? []} loading={items.isLoading} search={false} pagination={{ pageSize: 20 }} options={false} /></Drawer>
    <ModalForm<DecisionForm> form={decisionForm} key={deciding?.id ?? 'decision'} title="提交复核结果" open={Boolean(deciding)} onOpenChange={(value) => !value && setDeciding(undefined)} initialValues={{ decision: 'APPROVE' }} submitter={{ searchConfig: { submitText: '提交', resetText: '取消' } }} onFinish={async (values) => { await decide.mutateAsync(values); return true }}>
      <ProFormRadio.Group name="decision" label="复核结果" options={[{ label: '保留', value: 'APPROVE' }, { label: '撤销', value: 'REVOKE' }]} rules={[{ required: true }]} />
      <ProFormTextArea name="reason" label="原因" fieldProps={{ maxLength: 500, showCount: true }} rules={[{ validator: async (_: unknown, value: unknown) => { if (decisionForm.getFieldValue('decision') === 'REVOKE' && String(value ?? '').trim().length < 8) throw new Error('撤销角色时请填写至少 8 个字的原因') } }]} />
    </ModalForm>
  </PageContainer>
}
