import { useState } from 'react'
import { usePageTitle } from '../../hooks/usePageTitle'
import { App as AntdApp, Button, Form, Input, InputNumber, Modal, Popconfirm, Space, Table, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { adminCreateTag, adminDeleteTag, adminUpdateTag, listTags, queryKeys } from '../../api/api'
import type { Tag } from '../../types'

export default function AdminTags() {
  usePageTitle('标签管理')

  const { message } = AntdApp.useApp()
  const queryClient = useQueryClient()
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Tag | null>(null)
  const [form] = Form.useForm<Partial<Tag>>()

  const { data: tags, isLoading } = useQuery({ queryKey: queryKeys.tags, queryFn: listTags })

  const invalidate = () => void queryClient.invalidateQueries({ queryKey: queryKeys.tags })

  const saveMutation = useMutation({
    mutationFn: (input: Partial<Tag>) => (editing ? adminUpdateTag(editing.id, input) : adminCreateTag(input)),
    onSuccess: () => {
      message.success(editing ? '标签已更新' : '标签已创建')
      setModalOpen(false)
      setEditing(null)
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '保存失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => adminDeleteTag(id),
    onSuccess: () => {
      message.success('标签已删除')
      invalidate()
    },
    onError: (err) => message.error(err instanceof Error ? err.message : '删除失败'),
  })

  const openModal = (tag?: Tag) => {
    setEditing(tag ?? null)
    form.resetFields()
    if (tag) form.setFieldsValue(tag)
    else form.setFieldsValue({ sort: 0 })
    setModalOpen(true)
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 16 }}>
        <div>
          <Typography.Title level={3} style={{ marginBottom: 4 }}>
            标签管理
          </Typography.Title>
          <Typography.Paragraph style={{ color: 'var(--velora-text)', marginBottom: 0 }}>
            维护应用的标签体系，标签可跨分类组合筛选。
          </Typography.Paragraph>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>
          新建标签
        </Button>
      </div>

      <Table<Tag>
        rowKey="id"
        loading={isLoading}
        dataSource={tags ?? []}
        pagination={false}
        columns={[
          { title: '编码', dataIndex: 'code', width: 160 },
          { title: '名称', dataIndex: 'name', width: 180 },
          { title: '排序', dataIndex: 'sort', width: 80 },
          {
            title: '操作',
            key: 'actions',
            width: 140,
            render: (_, tag) => (
              <Space>
                <Button type="link" size="small" onClick={() => openModal(tag)}>
                  编辑
                </Button>
                <Popconfirm
                  title="删除标签"
                  description="删除后相关应用的该标签关系将一并移除。"
                  okText="删除"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => deleteMutation.mutate(tag.id)}
                >
                  <Button type="link" size="small" danger>
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? '编辑标签' : '新建标签'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
        destroyOnHidden
      >
        <Form
          form={form}
          layout="vertical"
          requiredMark={false}
          style={{ marginTop: 16 }}
          onFinish={(values) => saveMutation.mutate(values)}
        >
          <Form.Item label="标签编码" name="code" rules={[{ required: true, message: '请输入编码' }]}>
            <Input placeholder="如 ci / monitor / genai" />
          </Form.Item>
          <Form.Item label="标签名称" name="name" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如 CI/CD" />
          </Form.Item>
          <Form.Item label="排序（越小越靠前）" name="sort">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
