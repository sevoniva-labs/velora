import { useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import QueryErrorState from '../../components/QueryErrorState'
import AdminPageHead from '../../components/AdminPageHead'
import { App as AntdApp, Alert, Button, Form, Input, Modal, Popconfirm, Select, Table, Tag, Typography } from 'antd'
import { KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createIntegrationToken, listIntegrationTokens, revokeIntegrationToken, type IntegrationToken } from '../../api/api'

const { Text } = Typography

const SCOPE_OPTIONS = [
  { value: 'todo:write', label: '待办推送（todo:write）' },
]

export default function AdminIntegrationTokens() {
  usePageTitle('集成令牌')

  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [created, setCreated] = useState<{ token: string; message: string } | null>(null)
  const [form] = Form.useForm<{ name: string; scopes: string[]; expiresInDays?: number }>()

  const { data: tokens, isLoading, isError, refetch } = useQuery({
    queryKey: ['admin', 'integration-tokens'],
    queryFn: listIntegrationTokens,
  })

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: ['admin', 'integration-tokens'] })

  const createMutation = useMutation({
    mutationFn: (input: { name: string; scopes: string[]; expiresInDays?: number }) => createIntegrationToken(input),
    onSuccess: (r) => {
      setCreated({ token: r.token, message: r.message })
      setModalOpen(false)
      form.resetFields()
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '创建失败'),
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeIntegrationToken(id),
    onSuccess: () => {
      message.success('令牌已吊销')
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '吊销失败'),
  })

  return (
    <div>
      <AdminPageHead
        title="集成令牌"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
            新建令牌
          </Button>
        }
      />

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="Service Account 鉴权"
        description="供外部系统（如工单/CI 系统）以 Authorization: Bearer 调用集成端点（当前支持待办推送 POST /api/v1/todos），与用户会话解耦。令牌明文仅创建时展示一次，请立即保存。"
      />

      {created && (
        <Alert
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
          closable
          onClose={() => setCreated(null)}
          message="令牌已创建（仅展示一次，请立即复制保存）"
          description={
            <div>
              <Text code copyable>{created.token}</Text>
              <div style={{ marginTop: 4, color: '#fa8c16' }}>⚠ 关闭或刷新页面后将无法再次查看；遗失需吊销后重建。</div>
            </div>
          }
        />
      )}

      {isError ? (
        <QueryErrorState refetch={refetch} />
      ) : (
        <Table<IntegrationToken>
          rowKey="id"
          loading={isLoading}
          dataSource={tokens ?? []}
          pagination={false}
          locale={{ emptyText: '暂无集成令牌' }}
          columns={[
            { title: '名称', dataIndex: 'name', width: 200 },
            {
              title: '权限',
              key: 'scopes',
              render: (_, t) => <Tag color="blue">{t.scopes?.join(', ') || '-'}</Tag>,
            },
            {
              title: '创建者',
              dataIndex: 'createdBy',
              width: 140,
              render: (v: string) => v || '-',
            },
            {
              title: '过期时间',
              dataIndex: 'expiresAt',
              width: 120,
              render: (v: string | null) => (v ? new Date(v).toLocaleDateString() : '永不过期'),
            },
            {
              title: '最近使用',
              dataIndex: 'lastUsedAt',
              width: 120,
              render: (v: string | null) => (v ? new Date(v).toLocaleString() : '未使用'),
            },
            {
              title: '状态',
              dataIndex: 'revoked',
              width: 90,
              render: (v: boolean) =>
                v ? <Tag color="red">已吊销</Tag> : <Tag color="green">有效</Tag>,
            },
            {
              title: '操作',
              key: 'op',
              width: 100,
              render: (_, t) =>
                t.revoked ? null : (
                  <Popconfirm
                    title="吊销此令牌？"
                    description="吊销后使用该令牌的集成调用将立即返回 401。"
                    okText="吊销"
                    okButtonProps={{ danger: true }}
                    onConfirm={() => revokeMutation.mutate(t.id)}
                  >
                    <Button type="link" size="small" danger loading={revokeMutation.isPending}>
                      吊销
                    </Button>
                  </Popconfirm>
                ),
            },
          ]}
        />
      )}

      <Modal
        title="新建集成令牌"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => void form.validateFields().then((v) => createMutation.mutate(v))}
        confirmLoading={createMutation.isPending}
        okText="创建"
        width={520}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" requiredMark={false} style={{ marginTop: 4 }}>
          <Form.Item label="令牌名称" name="name" rules={[{ required: true, message: '请输入令牌名称（如：工单系统）' }]}>
            <Input placeholder="标识归属系统，如：工单系统" prefix={<KeyOutlined />} />
          </Form.Item>
          <Form.Item label="权限范围" name="scopes" rules={[{ required: true, message: '至少选择一个权限' }]}>
            <Select mode="multiple" placeholder="选择权限范围" options={SCOPE_OPTIONS} />
          </Form.Item>
          <Form.Item label="有效期（天）" name="expiresInDays" extra="留空表示永不过期；建议生产环境设置有效期">
            <Input type="number" min={1} placeholder="留空 = 永不过期" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
